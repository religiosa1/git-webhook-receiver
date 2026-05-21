package admin

import (
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type GetPipelineOutput struct {
	DB *actionsdb.ActionDB
}

func (s GetPipelineOutput) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	if s.DB == nil {
		logger.Error("pipeline output page accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	output, err := s.DB.GetPipelineOutput(pipeID)
	if err != nil {
		if mapError(err) == http.StatusInternalServerError {
			logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		}
		if writeErr := renderErr(w, req, err); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelineOutputPartial(string(output)).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
	} else {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write(output); err != nil {
			logger.Error("Error writing output", slog.Any("error", err))
		}
	}
}
