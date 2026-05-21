package admin

import (
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/a-h/templ"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

func parsePipelineFilterQuery(queryParams url.Values) (actionsdb.ListPipelineRecordsQuery, error) {
	query := actionsdb.ListPipelineRecordsQuery{
		Offset:     0,
		Limit:      0,
		Project:    queryParams.Get("project"),
		DeliveryID: queryParams.Get("deliveryId"),
		Cursor:     queryParams.Get("cursor"),
	}
	var err error
	query.Status, err = actionsdb.ParsePipelineStatus(queryParams.Get("status"))
	return query, err
}

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

type GetPipeline struct {
	DB *actionsdb.ActionDB
}

func (s GetPipeline) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	if s.DB == nil {
		logger.Error("pipeline page accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	record, err := s.DB.GetPipelineRecord(pipeID)
	if err != nil {
		if mapError(err) == http.StatusInternalServerError {
			logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		}
		if writeErr := renderErr(w, req, err); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	viewModel := views.PipelineItemViewModel{
		Record: record,
	}
	if err := views.PipelineItem(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

// TODO: support for live streaming in UI

type GetPipelineOutput struct {
	DB *actionsdb.ActionDB
}

func (s GetPipelineOutput) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	if s.DB == nil {
		logger.Error("pipeline output page accessed, while no actions db is provided")
		w.WriteHeader(http.StatusNotFound)
		if writeErr := views.NotFound().Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	output, err := s.DB.GetPipelineOutput(pipeID)
	if err != nil {
		if mapError(err) == http.StatusInternalServerError {
			logger.Error("Error processing pipeline ui request", slog.Any("error", err))
		}
		if writeErr := renderErr(w, req, err); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelineOutputPartial(string(output)).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
	} else {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write(output); err != nil {
			logger.Error("Error writing output", slog.Any("error", err))
		}
	}
}

func mapError(err error) int {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case errors.Is(err, actionsdb.ErrBadCursor), errors.Is(err, actionsdb.ErrCursorAndOffset):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func renderErr(w http.ResponseWriter, req *http.Request, err error) error {
	statusCode := mapError(err)
	w.WriteHeader(statusCode)
	var errView templ.Component
	switch statusCode {
	case http.StatusNotFound:
		errView = views.NotFound()
	case http.StatusBadRequest:
		errView = views.BadRequest(err)
	default:
		requestID := middleware.GetRequestID(req.Context())
		errView = views.InternalError(requestID)
	}
	return errView.Render(req.Context(), w)
}
