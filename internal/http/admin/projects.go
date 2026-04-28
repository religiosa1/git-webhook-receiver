package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
)

type ListProjects struct {
	Projects map[string]config.Project
}

func (h ListProjects) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(h.Projects)
	if err != nil {
		logger.Error("Error writing projects response", slog.Any("error", err))
	}
}

type GetProject struct {
	Project config.Project
}

func (h GetProject) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())

	w.Header().Set("Content-Type", "application/json")
	err := json.NewEncoder(w).Encode(h.Project)
	if err != nil {
		logger.Error("Error writing project response", slog.Any("error", err))
	}
}
