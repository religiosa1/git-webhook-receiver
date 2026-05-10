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
	queryParams := req.URL.Query()
	query := actionsdb.ListPipelineRecordsQuery{
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		Cursor:     queryParams.Get("cursor"),
	}
	var err error
	query.Status, err = actionsdb.ParsePipelineStatus(queryParams.Get("status"))
	if err != nil {
		logger.Warn("Error parsing pipeline state", slog.Any("error", err))
		// just logging out, no execution abort here
	}

	page, err := s.DB.ListPipelineRecords(query)
	if err != nil {
		statusCode := 500
		if errors.Is(err, actionsdb.ErrBadCursor) || errors.Is(err, actionsdb.ErrCursorAndOffset) {
			statusCode = 400
		}
		w.WriteHeader(statusCode)
		// TODO:
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
	}

	viewModel := views.PipelinesListViewModel{
		Page: page,
	}
	if err := views.PipelinesList(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

type GetPipeline struct {
	DB *actionsdb.ActionDB
}

func (s GetPipeline) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	record, err := s.DB.GetPipelineRecord(pipeID)
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
	viewModel := views.PipelineItemViewModel{
		Record: record,
	}
	if err := views.PipelineItem(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

type GetPipelineOutput struct {
	DB *actionsdb.ActionDB
}

func (s GetPipelineOutput) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	record, err := s.DB.GetPipelineRecord(pipeID)
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
	viewModel := views.PipelineOutputViewModel{
		PipeID: pipeID,
		Output: record.Output.String,
	}
	if err := views.PipelineOutput(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}
