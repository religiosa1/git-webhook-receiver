package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	"github.com/religiosa1/webhook-receiver/internal/action_runner"
	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/http/handlers"
	"github.com/religiosa1/webhook-receiver/internal/logger"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

const errCodeCreate int = 2
const errCodeLogger = 3
const errCodeRun int = 4
const errCodeShutdown int = 5

func main() {
	configPath := getConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Unable to load configuration file, aborting: %s", err)
		os.Exit(errCodeCreate)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	//==========================================================================
	// Logger

	closableLogger, err := logger.SetupLogger(cfg.LogLevel, cfg.LogFile)
	if err != nil {
		log.Printf("Error setting up the logger: %s", err)
		os.Exit(errCodeLogger)
	}
	defer closableLogger.Close()
	logger := closableLogger.Logger
	logger.Debug("configuration loaded", slog.Any("config", cfg))

	//==========================================================================
	// Action runner
	actionsCh := make(chan action_runner.ActionArgs)
	actionsCtx, actionsCancel := context.WithCancel(context.Background())
	var actionsWg sync.WaitGroup
	go action_runner.Listen(actionsCtx, actionsCh, &actionsWg, cfg.ActionsOutputDir)

	//==========================================================================
	// HTTP-Server
	srv, err := createServer(actionsCh, cfg, logger)
	if err != nil {
		logger.Error("Error creating the server", slog.Any("error", err))
		os.Exit(errCodeCreate)
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
			os.Exit(errCodeShutdown)
		} else {
			os.Exit(errCodeRun)
		}
	}
	logger.Info("Server closed")

	logger.Info("Waiting for actions to complete... Press ctrl+c again to forcefully close")
	go func() {
		select {
		case <-actionsCtx.Done():
			fmt.Println("Action completed")
		case <-interrupt:
			actionsCancel()
			fmt.Println("Action interrupted")
		}
	}()
	actionsWg.Wait()
	actionsCancel()

	logger.Info("Done")
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

func createServer(actionsCh chan action_runner.ActionArgs, cfg *config.Config, logger *slog.Logger) (*http.Server, error) {
	mux := http.NewServeMux()
	for projectName, project := range cfg.Projects {
		receiver := whreceiver.New(&project)
		if receiver == nil {
			return nil, fmt.Errorf("unknown git webhook provider type '%s' in project '%s'", project.GitProvider, projectName)
		}
		projectLogger := logger.With(slog.String("project", projectName))
		mux.HandleFunc(
			fmt.Sprintf("POST /%s", projectName),
			handlers.HandleWebhookPost(actionsCh, projectLogger, cfg, &project, receiver),
		)
		logger.Debug("Registered project",
			slog.String("projectName", projectName),
			slog.String("type", project.GitProvider),
			slog.String("repo", project.Repo),
		)
	}
	srv := &http.Server{
		Addr:    fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
		Handler: mux,
	}
	return srv, nil
}

func getConfigPath() string {
	var configPath string
	flag.StringVar(&configPath, "config", "", "Configuration file name")
	flag.Parse()

	if configPath == "" {
		configPath = os.Getenv("CONFIG_PATH")
	}
	if configPath == "" {
		configPath = "config.yml"
	}
	return configPath
}

type ErrShutdown struct {
	err error
}

func (e ErrShutdown) Error() string {
	return fmt.Sprintf("error shutting down the server: %s", e.err)
}
