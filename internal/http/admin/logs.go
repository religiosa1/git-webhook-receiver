package admin

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

func GetLogs(db *logsDb.LogsDB, logger *slog.Logger, publicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		queryParams := req.URL.Query()

		offset, _ := strconv.Atoi(queryParams.Get("offset"))
		limit, _ := strconv.Atoi(queryParams.Get("limit"))

		query := logsDb.GetEntryFilteredQuery{
			PageSize:   limit,
			Cursor:     queryParams.Get("cursor"),
			Levels:     parseLevels(queryParams["level"]),
			Project:    queryParams.Get("project"),
			DeliveryID: queryParams.Get("deliveryId"),
			PipeID:     queryParams.Get("pipeId"),
			Message:    queryParams.Get("message"),
			Offset:     offset,
		}

		page, err := db.GetEntryFiltered(query)
		if err != nil {
			statusCode := 500
			if errors.Is(err, logsDb.ErrBadCursor) || errors.Is(err, logsDb.ErrCursorAndOffset) {
				statusCode = 400
			}
			logger.Error("Error processing GetLogs request", slog.Any("error", err))
			if writeErr := utils.WriteErrorResponse(w, statusCode, err.Error()); writeErr != nil {
				logger.Error("error while writing error message", slog.Any("error", writeErr))
			}
			return
		}

		output := serialization.LogEntriesPage(page)
		output.NextPage = utils.BuildNextPageURL(req, publicURL, page.Cursor)
		err = json.NewEncoder(w).Encode(output)
		if err != nil {
			logger.Error("Error writing output", slog.Any("error", err))
		}
	}
}

func parseLevels(levels []string) []int {
	result := make([]int, 0)
	for _, lvl := range levels {
		l, err := logsDb.ParseLogLevel(lvl)
		if err == nil {
			result = append(result, l)
		}
	}
	return result
}
