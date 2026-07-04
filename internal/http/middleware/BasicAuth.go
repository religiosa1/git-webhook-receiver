package middleware

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/cryptoutils"
)

func WithBasicAuth(expectedUsername string, expectedPassword string, realm string) Middleware {
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
				// always do both, to make user enumeration harder
				if userOk, passOk := userNameComparer.Eq(username), passwordComparer.Eq(password); userOk && passOk {
					next.ServeHTTP(w, r)
					return
				}
				if logger != nil {
					logger.Info("Basic auth failed", slog.String("url", r.RequestURI), slog.String("method", r.Method), slog.String("remoteAddr", r.RemoteAddr))
				}
			}
			w.Header().Set("WWW-Authenticate", fmt.Sprintf(`Basic realm=%q, charset="UTF-8"`, realm))
			http.Error(w, "Unauthorized", http.StatusUnauthorized)
		})
	}
}

func noopHandler(next http.Handler) http.Handler {
	return next
}
