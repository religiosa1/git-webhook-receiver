package admin

import (
	"database/sql"
	"errors"
	"fmt"
	"html"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/a-h/templ"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/http/utils"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
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
	DB           *actionsdb.ActionDB
	TmpOutputMgr tmpoutput.Manager
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
	if req.Header.Get("HX-Request") == "true" {
		if err := views.PipelinePreviewPartial(record).Render(req.Context(), w); err != nil {
			logger.Error("Error while writing response", slog.Any("error", err))
		}
		return
	}
	_, isLive := s.TmpOutputMgr.Reader(req.Context(), pipeID)
	viewModel := views.PipelineItemViewModel{
		Record: record,
		IsLive: isLive,
	}
	if err := views.PipelineItem(viewModel).Render(req.Context(), w); err != nil {
		logger.Error("Error while writing response", slog.Any("error", err))
	}
}

type GetPipelineOutputStream struct {
	DB           *actionsdb.ActionDB
	TmpOutputMgr tmpoutput.Manager
}

func (s GetPipelineOutputStream) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	pipeID := req.PathValue("pipeId")
	logger := middleware.GetLogger(req.Context()).With(slog.String("pipe_id", pipeID))
	if s.DB == nil {
		http.Error(w, "Not Found", http.StatusNotFound)
		return
	}

	// Parse Last-Event-ID for reconnect resume — value is cumulative raw bytes sent.
	var offset int64
	if lastID := req.Header.Get("Last-Event-ID"); lastID != "" {
		offset, _ = strconv.ParseInt(lastID, 10, 64)
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("X-Accel-Buffering", "no")

	rc := http.NewResponseController(w)

	if reader, ok := s.TmpOutputMgr.Reader(req.Context(), pipeID); ok {
		// Skip already-delivered bytes on reconnect. liveReader reads from in-memory
		// data so this won't block as long as offset <= current buffer length.
		if offset > 0 {
			if _, err := io.CopyN(io.Discard, reader, offset); err != nil && !errors.Is(err, io.EOF) {
				logger.Error("error skipping SSE resume offset", slog.Any("error", err))
				return
			}
		}
		buf := make([]byte, 4096)
		for {
			n, err := reader.Read(buf)
			if n > 0 {
				offset += int64(n)
				if _, writeErr := fmt.Fprint(w, sseDataWithID(buf[:n], offset)); writeErr != nil {
					logger.Error("error writing SSE data", slog.Any("error", writeErr))
					return
				}
				if flushErr := rc.Flush(); flushErr != nil {
					logger.Error("error flushing SSE", slog.Any("error", flushErr))
					return
				}
			}
			if err == io.EOF {
				break
			}
			if err != nil {
				logger.Error("error reading pipeline output stream", slog.Any("error", err))
				return
			}
		}
	} else {
		// Pipeline already finished — serve remaining output from DB.
		output, err := s.DB.GetPipelineOutput(pipeID)
		if errors.Is(err, sql.ErrNoRows) {
			http.Error(w, "Not Found", http.StatusNotFound)
			return
		} else if err != nil {
			logger.Error("Error fetching pipeline output for SSE", slog.Any("error", err))
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		if int64(len(output)) > offset {
			remaining := output[offset:]
			offset += int64(len(remaining))
			if _, writeErr := fmt.Fprint(w, sseDataWithID(remaining, offset)); writeErr != nil {
				logger.Error("error writing SSE data", slog.Any("error", writeErr))
				return
			}
			_ = rc.Flush()
		}
	}

	if _, writeErr := fmt.Fprint(w, "event: done\ndata:\n\n"); writeErr != nil {
		logger.Error("error writing SSE done event", slog.Any("error", writeErr))
	}
	_ = rc.Flush()
}

// sseDataWithID formats a raw output chunk as SSE data lines with an id field.
// id is the cumulative raw byte offset, used by the client as Last-Event-ID on reconnect.
// Content is HTML-escaped for safe injection into a <pre> by htmx.
func sseDataWithID(b []byte, id int64) string {
	var sb strings.Builder
	sb.WriteString("id: ")
	sb.WriteString(strconv.FormatInt(id, 10))
	sb.WriteByte('\n')
	escaped := html.EscapeString(string(b))
	lines := strings.Split(escaped, "\n")
	for i, line := range lines {
		sb.WriteString("data: ")
		sb.WriteString(line)
		sb.WriteByte('\n')
		if i == len(lines)-1 {
			break
		}
	}
	sb.WriteByte('\n')
	return sb.String()
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
