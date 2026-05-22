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
