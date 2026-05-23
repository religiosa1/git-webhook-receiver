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
	w.Header().Set("X-Content-Type-Options", "nosniff")
	if output, ok := h.TmpOutputMgr.Reader(req.Context(), pipeID); ok {
		fw := flushWriter{w: w, f: http.NewResponseController(w)}
		if _, writeErr := io.Copy(fw, output); writeErr != nil && !errors.Is(writeErr, io.EOF) {
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

// flushWriter wraps ResponseWriter and flushes after each Write call,
// enabling live streaming to browsers without buffering.
type flushWriter struct {
	w http.ResponseWriter
	f *http.ResponseController
}

func (fw flushWriter) Write(p []byte) (n int, err error) {
	n, err = fw.w.Write(p)
	if err == nil {
		_ = fw.f.Flush()
	}
	return
}
