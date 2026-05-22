package cmd

import (
	"context"
	"fmt"
	"io/fs"
	"log"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionrunner"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/admin"
	"github.com/religiosa1/git-webhook-receiver/internal/http/api"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/webhook"
	"github.com/religiosa1/git-webhook-receiver/internal/logger"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

func Serve(cfg config.Config) {
	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	//==========================================================================
	// Logger and Action DBs

	dbActions, err := actionsdb.New(cfg.ActionsDBFile, cfg.MaxActionsStored)
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

	dbLogs, err := logsdb.New(cfg.LogsDBFile)
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

	logger, err := logger.SetupLogger(cfg.LogLevel, cfg.LogType, dbLogs)
	if err != nil {
		log.Printf("Error setting up the logger: %s", err)
		os.Exit(ExitCodeLoggerDB)
	}
	logger.Debug("configuration loaded", slog.Any("config", cfg))
	if cfg.PublicURL == "" {
		logger.Warn("PublicURL is not set in the config, URL generation in responses will be falling back to relative paths")
	}

	if dbActions != nil {
		n, err := dbActions.SweepStaleRecords()
		if err != nil {
			logger.Error("Error while sweeping stale DB records", slog.Any("error", err))
		} else if n != 0 {
			logger.Info("Marked stale pipeline records as errored", slog.Int64("n_records", n))
		} else {
			logger.Debug("No stale pipeline records found")
		}
	}

	tmpOutputMgr := tmpoutput.NewInMemoryTmpOutput(cfg.MaxOutputBytes)
	actionRunner := actionrunner.New(
		context.Background(),
		dbActions,
		tmpOutputMgr,
	)

	//==========================================================================
	// HTTP-Server
	mux, err := createProjectsMux(actionRunner.Chan(), cfg, logger)
	if err != nil {
		logger.Error("Error creating the server", slog.Any("error", err))
		os.Exit(ExitReadConfig)
	}
	if !cfg.DisableUI {
		middlewares := middleware.Chain(
			middleware.WithLogger(logger),
			middleware.WithBasicAuth(cfg.AuthUser, cfg.AuthPassword.RawContents()),
			views.WithBaseViewModel(cfg),
		)
		staticFS, err := fs.Sub(admin.StaticFiles, "static")
		if err != nil {
			logger.Error("Error setting up static files", slog.Any("error", err))
			os.Exit(ExitReadConfig)
		}
		mux.Handle("GET /static/", http.StripPrefix("/static/", http.FileServerFS(staticFS)))
		projectsPage := middlewares(admin.ListProjects{Projects: cfg.Projects, DB: dbActions})
		mux.Handle("GET /projects", projectsPage)
		if dbActions == nil && dbLogs == nil {
			mux.Handle("GET /", projectsPage)
		}
		projectNames := make([]string, 0, len(cfg.Projects))
		for name := range cfg.Projects {
			projectNames = append(projectNames, name)
		}
		if dbActions != nil {
			logger.Debug("Web admin enabled for pipelines")
			listPipelinesPage := middlewares(admin.ListPipelines{DB: dbActions, Projects: projectNames})
			mux.Handle("GET /", listPipelinesPage)
			mux.Handle("GET /pipelines", listPipelinesPage)
			mux.Handle("GET /pipelines/{pipeId}", middlewares(admin.GetPipeline{DB: dbActions, TmpOutputMgr: tmpOutputMgr}))
			mux.Handle("GET /pipelines/{pipeId}/output", middlewares(admin.GetPipelineOutput{DB: dbActions}))
			mux.Handle("GET /pipelines/{pipeId}/output/stream", middlewares(admin.GetPipelineOutputStream{DB: dbActions, TmpOutputMgr: tmpOutputMgr}))
		} else {
			logger.Info("actions_db_file config value is an empty string. All of /pipelines pages won't be available")
		}
		logsPage := middlewares(admin.GetLogs{DB: dbLogs, Projects: projectNames})
		if dbLogs != nil {
			logger.Debug("Web admin enabled for logs")
			if dbActions == nil {
				mux.Handle("GET /", logsPage)
			}
		} else {
			logger.Info("logs_db_file config value is an empty string. Logs page won't be available")
		}
		mux.Handle("GET /logs", logsPage)
	}
	if !cfg.DisableAPI {
		middlewares := middleware.Chain(
			middleware.WithLogger(logger),
			middleware.WithBasicAuth(cfg.AuthUser, cfg.AuthPassword.RawContents()),
		)
		mux.Handle("GET /api/projects", middlewares(api.ListProjects{Projects: cfg.Projects}))
		if dbActions != nil {
			logger.Debug("HTTP API enabled for pipelines")
			mux.Handle("GET /api/pipelines", middlewares(api.ListPipelines{DB: dbActions, PublicURL: cfg.PublicURL}))
			mux.Handle("GET /api/pipelines/{pipeId}", middlewares(api.GetPipeline{DB: dbActions}))
			mux.Handle("GET /api/pipelines/{pipeId}/output", middlewares(api.GetPipelineOutput{DB: dbActions, TmpOutputMgr: tmpOutputMgr}))
		} else {
			logger.Info("actions_db_file config value is an empty string. All of /api/pipelines API endpoints won't be available")
		}
		if dbLogs != nil {
			logger.Debug("HTTP API enabled for logs")
			mux.Handle("GET /api/logs", middlewares(api.GetLogs{DB: dbLogs, PublicURL: cfg.PublicURL}))
		} else {
			logger.Info("logs_db_file config value is an empty string. Logs inspection won't be available")
		}
	} else {
		mux.HandleFunc("GET /api/", http.NotFound)
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
			logger.Info(
				"Running the server with SSL",
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

func createProjectsMux(actionsCh chan actionrunner.ActionArgs, cfg config.Config, logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	basicAuth := middleware.WithBasicAuth(cfg.AuthUser, cfg.AuthPassword.RawContents())
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
		handler := webhook.Webhook{
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
				"GET /api"+path,
				middleware.WithLogger(projectLogger)(basicAuth(api.GetProject{Project: project})),
			)
		}
		logger.Debug(
			"Registered project",
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
		mux.Handle("GET /api/projects/{projectName}", basicAuth(http.HandlerFunc(prjNotFound)))
	}
	return mux, nil
}

type ErrShutdown struct {
	err error
}

func (e ErrShutdown) Error() string {
	return fmt.Sprintf("error shutting down the server: %s", e.err)
}
