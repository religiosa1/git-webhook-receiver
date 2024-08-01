package main

import (
	"flag"
	"fmt"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/http/handlers"
	"github.com/religiosa1/webhook-receiver/internal/logger"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

func main() {
	configPath := getConfigPath()
	cfg, err := config.Load(configPath)
	if err != nil {
		log.Fatalf("Unable to load configuration file, aborting: %s", err)
	}

	closableLogger := logger.SetupLogger(cfg.LogLevel, cfg.LogFile)
	defer closableLogger.Close()
	logger := closableLogger.Logger
	logger.Debug("configuration loaded", slog.Any("config", cfg))
	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go runServer(cfg, logger)

	<-done
	logger.Info("Server closed")
}

func runServer(cfg *config.Config, logger *slog.Logger) {
	mux := http.NewServeMux()
	for projectName, project := range cfg.Projects {
		receiver := whreceiver.New(&project)
		if receiver == nil {
			log.Fatalf("Unknown git webhook provider type '%s' in project '%s'", project.GitProvider, projectName)
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

	if cfg.Ssl.CertFilePath != "" && cfg.Ssl.KeyFilePath != "" {
		logger.Info("Running the server with SSL",
			slog.String("host", cfg.Host),
			slog.Int("port", int(cfg.Port)),
			slog.String("cert file", cfg.Ssl.CertFilePath),
			slog.String("key file", cfg.Ssl.KeyFilePath),
		)
		err := http.ListenAndServeTLS(
			fmt.Sprintf("%s:%d", cfg.Host, cfg.Port),
			cfg.Ssl.CertFilePath,
			cfg.Ssl.KeyFilePath,
			mux,
		)
		if err != nil {
			logger.Error("Error starting the server", slog.Any("error", err))
			os.Exit(1)
		}
	} else {
		logger.Info("Running the server", slog.String("host", cfg.Host), slog.Int("port", int(cfg.Port)))
		if err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), mux); err != nil {
			logger.Error("Error starting the server", slog.Any("error", err))
			os.Exit(1)
		}
	}
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
