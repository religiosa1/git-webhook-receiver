package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/ActionRunner"
	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/admin"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	handlers "github.com/religiosa1/git-webhook-receiver/internal/http/webhook_handlers"
	"github.com/religiosa1/git-webhook-receiver/internal/logger"
	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

func Serve(cfg config.Config) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	//==========================================================================
	// Logger and Action DBs

	dbActions, err := actiondb.New(cfg.ActionsDBFile, cfg.MaxActionsStored, cfg.MaxOutputBytes)
	if err != nil {
		log.Printf("Error opening actions db: %s", err)
		os.Exit(ExitCodeActionsDB)
	}
	defer func() {
		if dbActions != nil {
			err := dbActions.Close()
			if err != nil {
				log.Printf("Error closing actions db: %s", err)
			}
		}
	}()

	dbLogs, err := logsDb.New(cfg.LogsDBFile)
	if err != nil {
		log.Printf("Error opening logs db: %s", err)
		os.Exit(ExitCodeLoggerDB)
	}
	defer func() {
		if dbLogs != nil {
			err := dbLogs.Close()
			if err != nil {
				log.Printf("Error closing logs db: %s", err)
			}
		}
	}()

	logger, err := logger.SetupLogger(cfg.LogLevel, dbLogs)
	if err != nil {
		log.Printf("Error setting up the logger: %s", err)
		os.Exit(ExitCodeLoggerDB)
	}
	logger.Debug("configuration loaded", slog.Any("config", cfg))
	if cfg.PublicURL == "" {
		logger.Warn("PublicURL is not set in the config, URL generation in responses will be falling back to relative paths")
	}

	actionRunner := ActionRunner.New(
		context.Background(),
		dbActions,
	)

	//==========================================================================
	// HTTP-Server
	mux, err := createProjectsMux(actionRunner.Chan(), cfg, logger)
	if err != nil {
		logger.Error("Error creating the server", slog.Any("error", err))
		os.Exit(ExitReadConfig)
	}
	if !cfg.DisableAPI {
		middlewares := middleware.Chain(
			middleware.WithLogger(logger),
			middleware.WithBasicAuth(cfg.APIUser, cfg.APIPassword.RawContents()),
		)
		mux.Handle("GET /projects", middlewares(admin.ListProjects{Projects: cfg.Projects}))
		if dbActions != nil {
			logger.Debug("Web admin enabled for pipelines")
			mux.Handle("GET /pipelines", middlewares(admin.ListPipelines{DB: dbActions, PublicURL: cfg.PublicURL}))
			mux.Handle("GET /pipelines/{pipeId}", middlewares(admin.GetPipeline{DB: dbActions}))
			mux.Handle("GET /pipelines/{pipeId}/output", middlewares(admin.GetPipelineOutput{DB: dbActions}))
		} else {
			logger.Warn("actions_db_file config value is an empty string. All of /pipeline API endpoints won't be available")
		}
		if dbLogs != nil {
			logger.Debug("Web admin enabled for logs")
			mux.Handle("GET /logs", middlewares(admin.GetLogs{DB: dbLogs, PublicURL: cfg.PublicURL}))
		} else {
			logger.Warn("logs_db_file config value is an empty string. Logs inspection won't be available")
		}
	}

	srv := &http.Server{
		Addr:    cfg.Addr,
		Handler: mux,
	}

	srvCtx, srcCancel := context.WithCancel(context.Background())
	defer srcCancel()
	go func() {
		<-interrupt
		srcCancel()
	}()

	if err := runServer(srvCtx, srv, cfg.Ssl, logger); err != nil {
		logger.Error("Error running the server", slog.Any("error", err))
		if _, ok := err.(ErrShutdown); ok {
			os.Exit(ExitCodeShutdown)
		} else {
			os.Exit(ExitCodeRun)
		}
	}
	logger.Info("Server closed")

	go func() {
		select {
		case <-actionRunner.Done():
			// Action completed successfully within the timeout.
		case <-time.After(500 * time.Millisecond):
			logger.Info("Waiting for actions to complete... Press ctrl+c again to forcefully close")
			<-actionRunner.Done()
		}
	}()
	go func() {
		select {
		case <-actionRunner.Done():
			logger.Info("Action runner closed")
		case <-interrupt:
			actionRunner.Cancel()
			logger.Warn("Actions interrupted")
		}
	}()
	actionRunner.Wait()
}

func runServer(ctx context.Context, srv *http.Server, sslConfig config.SslConfig, logger *slog.Logger) error {
	network, address := config.ParseAddr(srv.Addr)

	if network == "unix" {
		// remove stale socket file if present, silently erroring otherwise
		_ = os.Remove(address)
	}

	ln, err := net.Listen(network, address)
	if err != nil {
		return fmt.Errorf("failed to listen on %s: %w", srv.Addr, err)
	}

	if network == "unix" {
		defer func() {
			err := os.Remove(address)
			if err != nil {
				logger.Error("error removing unix socket path", slog.Any("error", err), slog.String("addr", srv.Addr))
			}
		}()
	}

	errCh := make(chan error, 1)
	go func() {
		var err error
		if sslConfig.CertFilePath != "" && sslConfig.KeyFilePath != "" {
			logger.Info("Running the server with SSL",
				slog.String("addr", srv.Addr),
				slog.String("cert file", sslConfig.CertFilePath),
				slog.String("key file", sslConfig.KeyFilePath),
			)
			err = srv.ServeTLS(ln, sslConfig.CertFilePath, sslConfig.KeyFilePath)
		} else {
			logger.Info("Running the server", slog.String("addr", srv.Addr))
			err = srv.Serve(ln)
		}
		if err == http.ErrServerClosed {
			err = nil
		}
		errCh <- err
	}()

	select {
	case err := <-errCh:
		return err
	case <-ctx.Done():
	}

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(ctxShutDown); err != nil {
		return ErrShutdown{err}
	}

	return <-errCh
}

func createProjectsMux(actionsCh chan ActionRunner.ActionArgs, cfg config.Config, logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	basicAuth := middleware.WithBasicAuth(cfg.APIUser, cfg.APIPassword.RawContents())
	for projectName, project := range cfg.Projects {
		receiver := whreceiver.New(project)
		if receiver == nil {
			return nil, fmt.Errorf("unknown git webhook provider type %q in project %q", project.GitProvider, projectName)
		}

		caps := receiver.GetCapabilities()
		if !caps.CanAuthorize && !project.Authorization.IsZero() {
			return nil, fmt.Errorf("misconfigured project %q, receiver %q does not support authorization, but it was provided", projectName, project.GitProvider)
		}
		if !caps.CanVerifySignature && !project.Secret.IsZero() {
			return nil, fmt.Errorf("misconfigured project %q, receiver %q does not support signature validation, but secret was provided", projectName, project.GitProvider)
		}

		projectLogger := logger.With(slog.String("project", projectName))
		path := fmt.Sprintf("/projects/%s", projectName)
		handler := handlers.Webhook{
			ActionsCh:   actionsCh,
			Config:      cfg,
			ProjectName: projectName,
			Project:     project,
			Receiver:    receiver,
		}
		mux.Handle(
			"POST "+path,
			middleware.WithLogger(projectLogger)(handler),
		)
		if !cfg.DisableAPI {
			mux.Handle(
				"GET "+path,
				middleware.WithLogger(projectLogger)(basicAuth(admin.GetProject{Project: project})),
			)
		}
		logger.Debug("Registered project",
			slog.String("projectName", projectName),
			slog.String("type", project.GitProvider),
			slog.String("repo", project.Repo),
		)
	}
	// fallback route, just for logging out errors
	prjNotFound := func(w http.ResponseWriter, req *http.Request) {
		projectName := req.PathValue("projectName")
		logger.Error("unknown git project passed", slog.String("project", projectName))
		w.WriteHeader(http.StatusNotFound)
	}
	mux.HandleFunc("POST /projects/{projectName}", prjNotFound)
	if !cfg.DisableAPI {
		mux.Handle("GET /projects/{projectName}", basicAuth(http.HandlerFunc(prjNotFound)))
	}
	return mux, nil
}

type ErrShutdown struct {
	err error
}

func (e ErrShutdown) Error() string {
	return fmt.Sprintf("error shutting down the server: %s", e.err)
}
