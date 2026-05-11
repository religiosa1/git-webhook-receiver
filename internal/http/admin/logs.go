package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type GetLogs struct {
	DB *logsdb.LogsDB
}

func (s GetLogs) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	if s.DB == nil {
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	queryParams := req.URL.Query()

	pagination, err := utils.ParsePagination(queryParams)
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	query := logsdb.GetEntryFilteredQuery{
		Limit:  pagination.Limit,
		Offset: pagination.Offset,
		Cursor: queryParams.Get("cursor"),
	}

	page, err := s.DB.GetEntryFiltered(query)
	if err != nil {
		statusCode := http.StatusInternalServerError
		if errors.Is(err, logsdb.ErrBadCursor) || errors.Is(err, logsdb.ErrCursorAndOffset) {
			statusCode = http.StatusBadRequest
		}
		logger.Error("Error processing logs ui request", slog.Any("error", err))
		w.WriteHeader(statusCode)
		requestID := middleware.GetRequestID(req.Context())
		if writeErr := views.InternalError(requestID).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	viewModel := views.LogsListViewModel{
		Page:     page,
		NextPage: utils.BuildNextPageURL(req, "", page.Cursor),
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.LogsListPartial(viewModel).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
		return
	}
	if err := views.LogsList(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}
