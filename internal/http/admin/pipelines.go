package admin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type ListPipelines struct {
	DB *actionsdb.ActionDB
}

func (s ListPipelines) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	queryParams := req.URL.Query()

	pagination, err := utils.ParsePagination(queryParams)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	query := actionsdb.ListPipelineRecordsQuery{
		Offset:     pagination.Offset,
		Limit:      pagination.Limit,
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		Cursor:     queryParams.Get("cursor"),
	}
	query.Status, err = actionsdb.ParsePipelineStatus(queryParams.Get("status"))
	if err != nil {
		logger.Warn("Error parsing pipeline state", slog.Any("error", err))
	}

	page, err := s.DB.ListPipelineRecords(query)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, actionsdb.ErrBadCursor) || errors.Is(err, actionsdb.ErrCursorAndOffset) {
			statusCode = http.StatusBadRequest
		}
		w.WriteHeader(statusCode)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	viewModel := views.PipelinesListViewModel{
		Page:     page,
		NextPage: utils.BuildNextPageURL(req, "", page.Cursor),
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelinesListPartial(viewModel).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
		return
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
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	} else if err != nil {
		logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
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
		w.WriteHeader(http.StatusNotFound)
		if req.Header.Get("HX-Request") != "true" {
			if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
				logger.Error("error while writing error response", slog.Any("error", writeErr))
			}
		}
		return
	} else if err != nil {
		logger.Error("Error processing pipeline output ui request", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		if req.Header.Get("HX-Request") != "true" {
			requestID := middleware.GetRequestID(req.Context())
			if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
				logger.Error("error while writing error response", slog.Any("error", writeErr))
			}
		}
		return
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelineOutputPartial(record.Output.String).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	if _, err := w.Write([]byte(record.Output.String)); err != nil {
		logger.Error("Error writing output", slog.Any("error", err))
	}
}
