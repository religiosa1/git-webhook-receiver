package middleware

import (
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/cryptoutils"
)

func WithBasicAuth(expectedUsername string, expectedPassword string) Middleware {
	if expectedUsername == "" || expectedPassword == "" {
		return noopHandler
	}

	userNameComparer := cryptoutils.NewConstantTimeComparer(expectedUsername)
	passwordComparer := cryptoutils.NewConstantTimeComparer(expectedPassword)

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			logger := GetLogger(r.Context())
			username, password, ok := r.BasicAuth()
			if !ok {
				if logger != nil {
					logger.Info("Basic auth required", slog.String("url", r.RequestURI), slog.String("method", r.Method), slog.String("remoteAddr", r.RemoteAddr))
				}
			} else {
				if userNameComparer.Eq(username) && passwordComparer.Eq(password) {
					next.ServeHTTP(w, r)
					return
				}
				if logger != nil {
					logger.Info("Basic auth failed", slog.String("url", r.RequestURI), slog.String("method", r.Method), slog.String("remoteAddr", r.RemoteAddr))
				}
			}
			w.Header().Set("WWW-Authenticate", `Basic realm="restricted", charset="UTF-8"`)
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

func noopHandler(next http.Handler) http.Handler {
	return next
}
