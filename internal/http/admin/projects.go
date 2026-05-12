package admin

import (
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type ListProjects struct {
	DB       *actionsdb.ActionDB
	Projects map[string]config.Project
}

func (l ListProjects) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	viewModel := views.ProjectsViewModel{
		Projects: l.Projects,
	}
	if err := views.Projects(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}
