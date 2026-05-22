package api

import (
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
