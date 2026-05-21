package admin

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type GetLogs struct {
	DB       *logsdb.LogsDB
	Projects []string
}

func parseLogsFilterQuery(queryParams url.Values) logsdb.GetEntryFilteredQuery {
	query := logsdb.GetEntryFilteredQuery{
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		PipeID:     queryParams.Get("pipeId"),
		Message:    queryParams.Get("message"),
		Cursor:     queryParams.Get("cursor"),
	}
	for _, lvl := range queryParams["level"] {
		if l, err := logsdb.ParseLogLevel(lvl); err == nil {
			query.Levels = append(query.Levels, l)
		}
	}
	return query
}

func (s GetLogs) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	if s.DB == nil {
		logger.Error("logs page accessed, while no logs db is provided")
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

	query := parseLogsFilterQuery(queryParams)
	query.Limit = pagination.Limit
	query.Offset = pagination.Offset

	page, err := s.DB.GetEntryFiltered(query)
	if err != nil {
		if mapError(err) == http.StatusInternalServerError {
			logger.Error("Error processing logs ui request", slog.Any("error", err))
		}
		if writeErr := renderErr(w, req, err); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	viewModel := views.LogsListViewModel{
		Page:     page,
		NextPage: utils.BuildNextPageURL(req, "", page.Cursor),
		Projects: s.Projects,
		Filter: views.LogsListFilter{
			Project:    query.Project,
			DeliveryID: query.DeliveryID,
			PipeID:     query.PipeID,
			Message:    query.Message,
			Levels:     queryParams["level"],
		},
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
