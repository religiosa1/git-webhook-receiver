package admin

import (
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type GetLogs struct {
	DB *logsdb.LogsDB
}

func (s GetLogs) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	if err := views.ToDo("Logs").Render(req.Context(), w); err != nil {
		logger.Error("error while writing the output", slog.Any("error", err))
	}
}
