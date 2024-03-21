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

	"github.com/religiosa1/deployer/internal/config"
	"github.com/religiosa1/deployer/internal/http/handlers"
	"github.com/religiosa1/deployer/internal/logger"
	"github.com/religiosa1/deployer/internal/wh_receiver"
)

func main() {
	configPath := getConfigPath()
	cfg := config.MustLoad(configPath)

	file, err := os.OpenFile(cfg.LogFile, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		panic(err)
	}
	defer file.Close()

	logger := logger.SetupLogger(cfg.LogLevel, file)
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
		receiver := wh_receiver.New(&project)
		if receiver == nil {
			log.Fatalf("Unknown git webhook provider type '%s' in project '%s'", project.GitProvider, projectName)
		}
		logger := logger.With(slog.String("project", projectName))
		mux.HandleFunc(
			fmt.Sprintf("POST /%s", projectName),
			handlers.HandleWebhookPost(logger, &project, receiver),
		)
		logger.Debug("Registered project", slog.String("projectName", projectName), slog.String("type", project.GitProvider), slog.String("repo", project.Repo))
	}

	logger.Info("Running the server", slog.String("host", cfg.Host), slog.Int("port", int(cfg.Port)))
	if err := http.ListenAndServe(fmt.Sprintf("%s:%d", cfg.Host, cfg.Port), mux); err != nil {
		logger.Error("Error starting the server", err)
		os.Exit(1)
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
