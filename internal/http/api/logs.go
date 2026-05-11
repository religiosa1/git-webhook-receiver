package api

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

type GetLogs struct {
	DB        *logsdb.LogsDB
	PublicURL string
}

func (h GetLogs) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	w.Header().Set("Content-Type", "application/json")
	queryParams := req.URL.Query()

	pagination, err := utils.ParsePagination(queryParams)
	if err != nil {
		if writeErr := utils.WriteErrorResponse(w, 400, err.Error()); writeErr != nil {
			logger.Error("error while writing error message", slog.Any("error", writeErr))
		}
		return
	}

	query := logsdb.GetEntryFilteredQuery{
		Limit:      pagination.Limit,
		Offset:     pagination.Offset,
		Cursor:     queryParams.Get("cursor"),
		Levels:     parseLevels(queryParams["level"]),
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		PipeID:     queryParams.Get("pipeId"),
		Message:    queryParams.Get("message"),
	}

	page, err := h.DB.GetEntryFiltered(query)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, logsdb.ErrBadCursor) || errors.Is(err, logsdb.ErrCursorAndOffset) {
			statusCode = http.StatusBadRequest
		}
		logger.Error("Error processing GetLogs request", slog.Any("error", err))
		if writeErr := utils.WriteErrorResponse(w, statusCode, err.Error()); writeErr != nil {
			logger.Error("error while writing error message", slog.Any("error", writeErr))
		}
		return
	}

	output := serialization.LogEntriesPage(page)
	output.NextPage = utils.BuildNextPageURL(req, h.PublicURL, page.Cursor)
	err = json.NewEncoder(w).Encode(output)
	if err != nil {
		logger.Error("Error writing output", slog.Any("error", err))
	}
}

func parseLevels(levels []string) []int {
	result := make([]int, 0)
	for _, lvl := range levels {
		l, err := logsdb.ParseLogLevel(lvl)
		if err == nil {
			result = append(result, l)
		}
	}
	return result
}
