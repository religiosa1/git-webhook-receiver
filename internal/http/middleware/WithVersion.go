package middleware

import (
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/version"
)

// WithVersion stamps the application build version onto every response, so it is
// observable from any endpoint (webhook, API, admin UI, static files).
func WithVersion() Middleware {
	v := version.String()
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Set before ServeHTTP: once the handler calls WriteHeader the
			// header map is frozen, so late additions would be dropped.
			w.Header().Set("X-Webhook-Receiver-Version", v)
			next.ServeHTTP(w, r)
		})
	}
}
