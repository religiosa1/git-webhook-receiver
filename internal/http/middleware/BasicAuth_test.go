package middleware_test

import (
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/http/middleware"
)

func TestBasicAuthMiddleware(t *testing.T) {
	logger := slog.New(slog.NewTextHandler(os.Stdout, nil))
	makeDummyHandler := func() http.HandlerFunc {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(http.StatusOK)
		})
	}

	t.Run("returns 401 if no auth is provided", func(t *testing.T) {
		handler := middleware.NewBasicAuth("validUser", "validPass", logger)(makeDummyHandler())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("returns 401 if incorrect auth is provided", func(t *testing.T) {
		handler := middleware.NewBasicAuth("validUser", "validPass", logger)(makeDummyHandler())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("wrongUser", "wrongPass")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusUnauthorized {
			t.Errorf("expected status 401, got %d", rr.Code)
		}
	})

	t.Run("forwards request to the handler function on ok request", func(t *testing.T) {
		handler := middleware.NewBasicAuth("validUser", "validPass", logger)(makeDummyHandler())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		req.SetBasicAuth("validUser", "validPass")
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})

	t.Run("immediately forwards request to the handler function if credentials are empty", func(t *testing.T) {
		handler := middleware.NewBasicAuth("", "", logger)(makeDummyHandler())

		req := httptest.NewRequest(http.MethodGet, "/", nil)
		rr := httptest.NewRecorder()

		handler.ServeHTTP(rr, req)

		if rr.Code != http.StatusOK {
			t.Errorf("expected status 200, got %d", rr.Code)
		}
	})
}
