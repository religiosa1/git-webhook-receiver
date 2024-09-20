package admin

import (
	"database/sql"
	"encoding/json"
	"log/slog"
	"net/http"
	"strconv"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

func ListPipelines(db *actiondb.ActionDb, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := req.URL.Query()

		offset, _ := strconv.Atoi(queryParams.Get("offset"))
		limit, _ := strconv.Atoi(queryParams.Get("limit"))

		query := actiondb.ListPipelineRecordsQuery{
			Offset:     offset,
			Limit:      limit,
			Project:    queryParams.Get("project"),
			DeliveryId: queryParams.Get("deliveryId"),
		}
		query.Status, _ = actiondb.ParsePipelineStatus(queryParams.Get("status"))

		records, err := db.ListPipelineRecords(query)
		if err != nil {
			w.WriteHeader(500)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serialization.PipelineRecords(records))
	}
}

func GetPipeline(db *actiondb.ActionDb, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pipeId := req.PathValue("pipeId")
		record, err := db.GetPipelineRecord(pipeId)
		if err == sql.ErrNoRows {
			w.WriteHeader(404)
			return
		} else if err != nil {
			logger.Error("Error processing GetPipeline request", slog.Any("error", err))
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(serialization.PipelineRecord(record))
	}
}

func GetPipelineOutput(db *actiondb.ActionDb, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pipeId := req.PathValue("pipeId")
		record, err := db.GetPipelineRecord(pipeId)
		if err == sql.ErrNoRows {
			w.WriteHeader(404)
			return
		} else if err != nil {
			logger.Error("Error processing GetPipelineOutput request", slog.Any("error", err))
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Write([]byte(record.Output.String))
	}
}
