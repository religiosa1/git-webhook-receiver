package admin

import (
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

type ListPipelines struct {
	DB       *actionsdb.ActionDB
	Projects []string
}

func (s ListPipelines) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	logger := middleware.GetLogger(req.Context())
	if s.DB == nil {
		logger.Error("pipelines page accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	query, err := parsePipelineFilterQuery(req.URL.Query())
	if err != nil {
		w.WriteHeader(http.StatusBadRequest)
		if writeErr := views.BadRequest(err).Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	page, err := s.DB.ListPipelineRecords(query)
	if err != nil {
		if mapError(err) == http.StatusInternalServerError {
			logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		}
		if writeErr := renderErr(w, req, err); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}

	viewModel := views.PipelinesListViewModel{
		Page:     page,
		NextPage: utils.BuildNextPageURL(req, "", page.Cursor),
		Projects: s.Projects,
		Filter: views.PipelinesListFilter{
			Project:    query.Project,
			DeliveryID: query.DeliveryID,
			Hash:       query.Hash,
			Status:     query.Status.String(),
		},
	}
	var view templ.Component
	if req.Header.Get("HX-Request") == "true" {
		view = views.PipelinesListPartial(viewModel)
	} else {
		view = views.PipelinesList(viewModel)
	}
	if err := view.Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

func parsePipelineFilterQuery(queryParams url.Values) (actionsdb.ListPipelineRecordsQuery, error) {
	query := actionsdb.ListPipelineRecordsQuery{
		Offset:     0,
		Limit:      0,
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		Hash:       queryParams.Get("hash"),
		Cursor:     queryParams.Get("cursor"),
	}
	var err error
	query.Status, err = actionsdb.ParsePipelineStatus(queryParams.Get("status"))
	return query, err
}
