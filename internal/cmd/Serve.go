package cmd

import (
	"context"
	"fmt"
	"log"
	"log/slog"
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

	dbActions, err := actiondb.New(cfg.ActionsDbFile)
	if err != nil {
		log.Printf("Error opening actions db: %s", err)
		os.Exit(ExitCodeActionsDb)
	}
	defer func() {
		if dbActions != nil {
			dbActions.Close()
		}
	}()

	dbLogs, err := logsDb.New(cfg.LogsDbFile)
	if err != nil {
		log.Printf("Error opening logs db: %s", err)
		os.Exit(ExitCodeLoggerDb)
	}
	defer func() {
		if dbLogs != nil {
			dbLogs.Close()
		}
	}()

	logger, err := logger.SetupLogger(cfg.LogLevel, dbLogs)
	if err != nil {
		log.Printf("Error setting up the logger: %s", err)
		os.Exit(ExitCodeLoggerDb)
	}
	logger.Debug("configuration loaded", slog.Any("config", cfg.MaskSensitiveData()))

	actionRunner := ActionRunner.New(context.Background(), dbActions)

	//==========================================================================
	// HTTP-Server
	mux, err := createProjectsMux(actionRunner.Chan(), cfg, logger)
	if err != nil {
		logger.Error("Error creating the server", slog.Any("error", err))
		os.Exit(ExitReadConfig)
	}
	if !cfg.DisableApi {
		if dbActions != nil {
			logger.Debug("Web admin enabled for pipelines")
			basicAuth := middleware.NewBasicAuth(cfg.ApiUser, cfg.ApiPassword, logger)
			mux.HandleFunc("GET /pipelines", basicAuth(admin.ListPipelines(dbActions, logger)))
			mux.HandleFunc("GET /pipelines/{pipeId}", basicAuth(admin.GetPipeline(dbActions, logger)))
			mux.HandleFunc("GET /pipelines/{pipeId}/output", basicAuth(admin.GetPipelineOutput(dbActions, logger)))
		} else {
			logger.Warn("actions_db_file config value is an empty string. All of /pipeline API endpoints won't be available")
		}
		if dbLogs != nil {
			logger.Debug("Web admin enabled for logs")
			mux.HandleFunc("GET /logs", admin.GetLogs(dbLogs, logger))
		} else {
			logger.Warn("logs_db_file config value is an empty string. Logs inspection won't be available")
		}
	}

	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
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
			logger.Info("Actions completed")
		case <-interrupt:
			actionRunner.Cancel()
			logger.Warn("Actions interrupted")
		}
	}()
	actionRunner.Wait()
	if dbActions != nil {
		dbActions.Close()
	}
	if dbLogs != nil {
		dbLogs.Close()
	}
}

func runServer(ctx context.Context, srv *http.Server, sslConfig config.SslConfig, logger *slog.Logger) (err error) {
	go func() {
		if sslConfig.CertFilePath != "" && sslConfig.KeyFilePath != "" {
			logger.Info("Running the server with SSL",
				slog.String("addr", srv.Addr),
				slog.String("cert file", sslConfig.CertFilePath),
				slog.String("key file", sslConfig.KeyFilePath),
			)
			err = srv.ListenAndServeTLS(sslConfig.CertFilePath, sslConfig.KeyFilePath)
		} else {
			logger.Info("Running the server", slog.String("addr", srv.Addr))
			err = srv.ListenAndServe()
		}
		if err == http.ErrServerClosed {
			err = nil
		}
	}()

	<-ctx.Done()

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = srv.Shutdown(ctxShutDown); err != nil {
		err = ErrShutdown{err}
	}

	return err
}

func createProjectsMux(actionsCh chan ActionRunner.ActionArgs, cfg config.Config, logger *slog.Logger) (*http.ServeMux, error) {
	mux := http.NewServeMux()
	for projectName, project := range cfg.Projects {
		receiver := whreceiver.New(project)
		if receiver == nil {
			return nil, fmt.Errorf("unknown git webhook provider type '%s' in project '%s'", project.GitProvider, projectName)
		}

		caps := receiver.GetCapabilities()
		if !caps.CanAuthorize && project.Authorization != "" {
			return nil, fmt.Errorf("misconfigured project '%s', receiver '%s' does not support authorization, but it was provided", projectName, project.GitProvider)
		}
		if !caps.CanVerifySignature && project.Secret != "" {
			return nil, fmt.Errorf("misconfigured project '%s', receiver '%s' does not support signature validation, but secret was provided", projectName, project.GitProvider)
		}

		projectLogger := logger.With(slog.String("project", projectName))
		mux.HandleFunc(
			fmt.Sprintf("POST /projects/%s", projectName),
			handlers.HandleWebhookPost(actionsCh, projectLogger, cfg, projectName, project, receiver),
		)
		logger.Debug("Registered project",
			slog.String("projectName", projectName),
			slog.String("type", project.GitProvider),
			slog.String("repo", project.Repo),
		)
	}
	return mux, nil
}

type ErrShutdown struct {
	err error
}

func (e ErrShutdown) Error() string {
	return fmt.Sprintf("error shutting down the server: %s", e.err)
}
