package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

type ListPipelines struct {
	DB        *actionsdb.ActionDB
	PublicURL string
}

func (h ListPipelines) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	queryParams := req.URL.Query()

	if h.DB == nil {
		logger.Error("pipelines endpoint accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := utils.WriteErrorResponse(w, http.StatusNotFound, "not found"); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	pagination, err := utils.ParsePagination(queryParams)
	if err != nil {
		if writeErr := utils.WriteErrorResponse(w, http.StatusBadRequest, err.Error()); writeErr != nil {
			logger.Error("error while writing error message", slog.Any("error", writeErr))
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
		// just logging out, no execution abort here
	}

	page, err := h.DB.ListPipelineRecords(query)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, actionsdb.ErrBadCursor) || errors.Is(err, actionsdb.ErrCursorAndOffset) {
			statusCode = http.StatusBadRequest
		}
		if writeErr := utils.WriteErrorResponse(w, statusCode, err.Error()); writeErr != nil {
			logger.Error("error while writing error message", slog.Any("error", writeErr))
		}
		return
	}

	output := serialization.PipelinePage(page)
	output.NextPage = utils.BuildNextPageURL(req, h.PublicURL, page.Cursor)
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		logger.Error("Error writing output", slog.Any("error", err))
	}
}

type GetPipeline struct {
	DB *actionsdb.ActionDB
}

func (h GetPipeline) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	pipeID := req.PathValue("pipeId")

	if h.DB == nil {
		logger.Error("pipeline endpoint accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := utils.WriteErrorResponse(w, http.StatusNotFound, "not found"); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	record, err := h.DB.GetPipelineRecord(pipeID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Error("Error processing GetPipeline request", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		_, err = w.Write([]byte(err.Error()))
		if err != nil {
			logger.Error("Error writing error output", slog.Any("error", err))
		}
		return
	}
	w.Header().Set("Content-Type", "application/json")
	err = json.NewEncoder(w).Encode(serialization.PipelineRecord(record))
	if err != nil {
		logger.Error("Error writing output", slog.Any("error", err))
	}
}

type GetPipelineOutput struct {
	DB *actionsdb.ActionDB
}

func (h GetPipelineOutput) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	pipeID := req.PathValue("pipeId")

	if h.DB == nil {
		logger.Error("pipeline output endpoint accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := utils.WriteErrorResponse(w, http.StatusNotFound, "not found"); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	record, err := h.DB.GetPipelineRecord(pipeID)
	if errors.Is(err, sql.ErrNoRows) {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	} else if err != nil {
		logger.Error("Error processing GetPipelineOutput request", slog.Any("error", err))
		w.WriteHeader(http.StatusInternalServerError)
		_, err := w.Write([]byte(err.Error()))
		if err != nil {
			logger.Error("Error writing error output", slog.Any("error", err))
		}
		return
	}
	w.Header().Set("Content-Type", "text/plain")
	_, err = w.Write([]byte(record.Output.String))
	if err != nil {
		logger.Error("Error writing output", slog.Any("error", err))
	}
}
