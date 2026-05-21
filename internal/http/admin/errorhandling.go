package admin

import (
	"database/sql"
	"errors"
	"net/http"

	"github.com/a-h/templ"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

func mapError(err error) int {
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return http.StatusNotFound
	case errors.Is(err, actionsdb.ErrBadCursor),
		errors.Is(err, actionsdb.ErrCursorAndOffset),
		errors.Is(err, logsdb.ErrBadCursor),
		errors.Is(err, logsdb.ErrCursorAndOffset):
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
