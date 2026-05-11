package admin

import (
	"errors"
	"log/slog"
	"net/http"

	"github.com/a-h/templ"
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
		if writeErr := views.BadRequest(err).Render(req.Context(), w); writeErr != nil {
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
		var errView templ.Component
		if errors.Is(err, logsdb.ErrBadCursor) || errors.Is(err, logsdb.ErrCursorAndOffset) {
			w.WriteHeader(http.StatusBadRequest)
			errView = views.BadRequest(err)
		} else {
			logger.Error("Error processing logs ui request", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			requestID := middleware.GetRequestID(req.Context())
			errView = views.InternalError(requestID)
		}
		if writeErr := errView.Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	viewModel := views.LogsListViewModel{
		Page:     page,
		NextPage: utils.BuildNextPageURL(req, "", page.Cursor),
	}
	var view templ.Component
	if req.Header.Get("HX-Request") == "true" {
		view = views.LogsListPartial(viewModel)
	} else {
		view = views.LogsList(viewModel)
	}
	if err := view.Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}
