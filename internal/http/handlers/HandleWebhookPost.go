package handlers

import (
	"log/slog"
	"net/http"

	"github.com/religiosa1/deployer/internal/config"
)

func HandleWebhookPost(logger *slog.Logger, cfg *config.Config) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {

	}
}
