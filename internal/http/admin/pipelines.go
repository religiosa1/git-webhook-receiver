package admin

import (
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/serialization"
)

func ListPipelines(db *actiondb.ActionDB, logger *slog.Logger, publicURL string) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		queryParams := req.URL.Query()

		offset, _ := strconv.Atoi(queryParams.Get("offset"))
		limit, _ := strconv.Atoi(queryParams.Get("limit"))

		query := actiondb.ListPipelineRecordsQuery{
			Offset:     offset,
			Limit:      limit,
			Project:    queryParams.Get("project"),
			DeliveryID: queryParams.Get("deliveryId"),
			Cursor:     queryParams.Get("cursor"),
		}
		var err error
		query.Status, err = actiondb.ParsePipelineStatus(queryParams.Get("status"))
		if err != nil {
			logger.Warn("Error parsing pipeline state", slog.Any("error", err))
			// just logging out, no execution abort here
		}

		pages, err := db.ListPipelineRecords(query)
		if err != nil {
			statusCode := 500
			if errors.Is(err, actiondb.ErrBadCursor) || errors.Is(err, actiondb.ErrCursorAndOffset) {
				statusCode = 400
			}
			if writeErr := utils.WriteErrorResponse(w, statusCode, err.Error()); writeErr != nil {
				logger.Error("error while writing error message", slog.Any("error", writeErr))
			}
			return
		}

		output := serialization.PipelinePage(pages)
		if pages.Cursor != nil {
			nextPage := buildNextPageURL(req, publicURL, *pages.Cursor)
			output.NextPage = &nextPage
		}
		w.Header().Set("Content-Type", "application/json")
		err = json.NewEncoder(w).Encode(output)
		if err != nil {
			logger.Error("Error writing output", slog.Any("error", err))
		}
	}
}

// TODO: merge with a similar function in webhook and move to utils
func buildNextPageURL(req *http.Request, publicURL string, cursor string) string {
	params := req.URL.Query()
	params.Set("cursor", cursor)
	params.Del("offset")

	var base string
	if publicURL != "" {
		base = strings.TrimRight(publicURL, "/") + req.URL.Path
	} else {
		scheme := "http"
		if req.TLS != nil {
			scheme = "https"
		}
		base = scheme + "://" + req.Host + req.URL.Path
	}
	return base + "?" + params.Encode()
}

func GetPipeline(db *actiondb.ActionDB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pipeID := req.PathValue("pipeId")
		record, err := db.GetPipelineRecord(pipeID)
		if err == sql.ErrNoRows {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		} else if err != nil {
			logger.Error("Error processing GetPipeline request", slog.Any("error", err))
			w.WriteHeader(500)
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
}

func GetPipelineOutput(db *actiondb.ActionDB, logger *slog.Logger) http.HandlerFunc {
	return func(w http.ResponseWriter, req *http.Request) {
		pipeID := req.PathValue("pipeId")
		record, err := db.GetPipelineRecord(pipeID)
		if err == sql.ErrNoRows {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		} else if err != nil {
			logger.Error("Error processing GetPipelineOutput request", slog.Any("error", err))
			w.WriteHeader(500)
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
}
