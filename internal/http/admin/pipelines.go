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
		var errView templ.Component
		if errors.Is(err, actionsdb.ErrBadCursor) || errors.Is(err, actionsdb.ErrCursorAndOffset) {
			w.WriteHeader(http.StatusBadRequest)
			errView = views.BadRequest(err)
		} else {
			logger.Error("error while getting a list of pipelines", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			requestID := middleware.GetRequestID(req.Context())
			errView = views.InternalError(requestID)
		}
		if writeErr := errView.Render(req.Context(), w); writeErr != nil {
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
		var errView templ.Component
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			errView = views.NotFound()
		} else {
			logger.Error("Error processing pipeline ui request", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			requestID := middleware.GetRequestID(req.Context())
			errView = views.InternalError(requestID)
		}
		if writeErr := errView.Render(req.Context(), w); writeErr != nil {
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
	record, err := s.DB.GetPipelineRecord(pipeID)
	if err != nil {
		var errView templ.Component
		if errors.Is(err, sql.ErrNoRows) {
			w.WriteHeader(http.StatusNotFound)
			errView = views.NotFound()
		} else {
			logger.Error("Error processing pipeline output ui request", slog.Any("error", err))
			w.WriteHeader(http.StatusInternalServerError)
			requestID := middleware.GetRequestID(req.Context())
			errView = views.InternalError(requestID)
		}
		if writeErr := errView.Render(req.Context(), w); writeErr != nil {
			logger.Error("error while writing error response", slog.Any("error", writeErr))
		}
		return
	}
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelineOutputPartial(record.Output.String).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
	} else {
		w.Header().Set("Content-Type", "text/plain")
		if _, err := w.Write([]byte(record.Output.String)); err != nil {
			logger.Error("Error writing output", slog.Any("error", err))
		}
	}
}
