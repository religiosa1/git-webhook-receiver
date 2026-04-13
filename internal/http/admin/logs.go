package admin

import (
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

func GetLogs(db *logsDb.LogsDB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		queryParams := req.URL.Query()

		offset, _ := strconv.Atoi(queryParams.Get("offset"))
		limit, _ := strconv.Atoi(queryParams.Get("limit"))

		cursorID, _ := strconv.ParseInt(queryParams.Get("cursorId"), 10, 64)
		cursorTS, _ := strconv.ParseInt(queryParams.Get("cursorTs"), 10, 64)

		query := logsDb.GetEntryFilteredQuery{
			GetEntryQuery: logsDb.GetEntryQuery{
				CursorID: cursorID,
				CursorTS: cursorTS,
				PageSize: limit,
			},
			Levels:     parseLevels(queryParams["level"]),
			Project:    queryParams.Get("project"),
			DeliveryID: queryParams.Get("deliveryId"),
			PipeID:     queryParams.Get("pipeId"),
			Message:    queryParams.Get("message"),
			Offset:     offset,
		}

		logs, err := db.GetEntryFiltered(query)
		if err != nil {
			logger.Error("Error processing GetLogs request", slog.Any("error", err))
			w.WriteHeader(500)
			_, _ = w.Write([]byte(err.Error()))
			return
		}

		_ = json.NewEncoder(w).Encode(serialization.LogEntries(logs))
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
