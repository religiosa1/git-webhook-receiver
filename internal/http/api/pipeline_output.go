package api

import (
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

type GetPipelineOutput struct {
	DB           *actionsdb.ActionDB
	TmpOutputMgr tmpoutput.Manager
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

	w.Header().Set("Content-Type", "text/plain")
	if output, ok := h.TmpOutputMgr.Reader(req.Context(), pipeID); ok {
		if _, writeErr := io.Copy(w, output); writeErr != nil {
			logger.Error("error while streaming the output", slog.Any("error", writeErr))
		}
		return
	}

	output, err := h.DB.GetPipelineOutput(pipeID)
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
	if len(output) == 0 {
		w.WriteHeader(http.StatusNoContent)
		return
	}
	if _, writeErr := w.Write(output); writeErr != nil {
		logger.Error("error while writing the output", slog.Any("error", writeErr))
	}
}
