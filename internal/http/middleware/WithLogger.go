package middleware

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/oklog/ulid/v2"
)

type loggingContextKey string

const (
	loggingContextLogger = loggingContextKey("logging_context.logger")
)

func WithLogger(logger *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := ulid.Make().String()
			method := r.Method
			path := r.URL.Path
			newLogger := logger.With(
				slog.String("request_id", id),
				slog.String("method", method),
				slog.String("path", path),
			)

			ctx := context.WithValue(r.Context(), loggingContextLogger, newLogger)

			newLogger.Debug("Incoming request")
			t1 := time.Now()
			rw := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(rw, r.WithContext(ctx))

			var logFn func(msg string, args ...any)
			if rw.status >= 400 {
				logFn = newLogger.Warn
			} else {
				logFn = newLogger.Debug
			}
			logFn(
				"request completed",
				slog.String("duration", time.Since(t1).String()),
				slog.Int("status_code", rw.status),
			)
		})
	}
}

// Wrapper around the response writer, to capture the response code
type responseWriter struct {
	http.ResponseWriter
	status int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.status = code
	rw.ResponseWriter.WriteHeader(code)
}

func GetLogger(ctx context.Context) *slog.Logger {
	logger, ok := ctx.Value(loggingContextLogger).(*slog.Logger)
	if !ok {
		logger = slog.New(slog.NewTextHandler(io.Discard, nil))
	}
	return logger
}
