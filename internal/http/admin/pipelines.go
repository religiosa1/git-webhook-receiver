package admin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type ListPipelines struct {
	DB *actionsdb.ActionDB
}

func (s ListPipelines) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	if err := views.ToDo("Pipelines list").Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

type GetPipeline struct {
	DB *actionsdb.ActionDB
}

func (s GetPipeline) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	_, err := s.DB.GetPipelineRecord(pipeID)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(404)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	} else if err != nil {
		logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		w.WriteHeader(500)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	if err := views.ToDo("Pipeline").Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

type GetPipelineOutput struct {
	DB *actionsdb.ActionDB
}

func (s GetPipelineOutput) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	_, err := s.DB.GetPipelineRecord(pipeID)
	if errors.Is(err, sql.ErrNoRows) {
		w.WriteHeader(404)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	} else if err != nil {
		logger.Error("Error processing pipeline output ui request", slog.Any("error", err))
		w.WriteHeader(500)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	if err := views.ToDo("Pipeline output").Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}
