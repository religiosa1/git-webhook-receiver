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
	"syscall"
	"time"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/http/handlers"
	"github.com/religiosa1/webhook-receiver/internal/logger"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

const errCodeCreate int = 2
const errCodeRun int = 3
const errCodeShutdown int = 4

func main() {
	configPath := getConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Printf("Unable to load configuration file, aborting: %s", err)
		os.Exit(errCodeCreate)
	}

	interrupt := make(chan os.Signal, 1)
	signal.Notify(interrupt, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	closableLogger := logger.SetupLogger(cfg.LogLevel, cfg.LogFile)
	defer closableLogger.Close()
	logger := closableLogger.Logger
	logger.Debug("configuration loaded", slog.Any("config", cfg))

	srv, err := createServer(cfg, logger)
	if err != nil {
		logger.Error("Error creating the server", slog.Any("error", err))
		os.Exit(errCodeCreate)
	}

	go runServer(srv, cfg.Ssl, logger)

	<-interrupt

	ctxShutDown, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err = srv.Shutdown(ctxShutDown); err != nil {
		logger.Error("server Shutdown Failed", slog.Any("error", err))
		os.Exit(errCodeShutdown)
	}

	logger.Info("Server closed")
}

func runServer(srv *http.Server, sslConfig config.SslConfig, logger *slog.Logger) {
	var err error
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
	if err != nil && err != http.ErrServerClosed {
		logger.Error("Error starting the server", slog.Any("error", err))
		os.Exit(errCodeRun)
	}
}

func createServer(cfg *config.Config, logger *slog.Logger) (*http.Server, error) {
	mux := http.NewServeMux()
	for projectName, project := range cfg.Projects {
		receiver := whreceiver.New(&project)
		if receiver == nil {
			return nil, fmt.Errorf("unknown git webhook provider type '%s' in project '%s'", project.GitProvider, projectName)
		}
		projectLogger := logger.With(slog.String("project", projectName))
		mux.HandleFunc(
			fmt.Sprintf("POST /%s", projectName),
			handlers.HandleWebhookPost(projectLogger, cfg, &project, receiver),
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
