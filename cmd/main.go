package main

import (
	"flag"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"

	"github.com/religiosa1/deployer/internal/config"
	"github.com/religiosa1/deployer/internal/http/handlers"
	"github.com/religiosa1/deployer/internal/logger"
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

	done := make(chan os.Signal, 1)
	signal.Notify(done, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)

	go runServer(cfg, logger)
	logger.Info("Server is listening", slog.String("host", cfg.Host), slog.Int("port", int(cfg.Port)))

	<-done
	logger.Info("Server closed")
}

func runServer(cfg *config.Config, logger *slog.Logger) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /", handlers.HandleWebhookPost(logger, cfg))

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
