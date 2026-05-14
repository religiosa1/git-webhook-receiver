package views

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type baseViewModel struct {
	hasLogsPages     bool
	hasPipelinePages bool
	publicURL        string
}

type viewModelContextKey string

var baseViewModelContextKey viewModelContextKey = "baseViewModelContext"

func GetBaseViewModel(ctx context.Context) baseViewModel {
	if model, ok := ctx.Value(baseViewModelContextKey).(baseViewModel); ok {
		return model
	}
	return baseViewModel{}
}

func MakePublicURL(ctx context.Context, relative string) string {
	publicURL := GetBaseViewModel(ctx).publicURL
	if publicURL == "" {
		publicURL = "/"
	}
	path, query, hasQuery := strings.Cut(relative, "?")
	result, err := url.JoinPath(publicURL, path)
	if err != nil {
		return relative
	}
	if hasQuery {
		result += "?" + query
	}
	return result
}

func WithBaseViewModel(cfg config.Config) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		model := baseViewModel{
			hasLogsPages:     cfg.LogsDBFile != "",
			hasPipelinePages: cfg.ActionsDBFile != "",
			publicURL:        cfg.PublicURL,
		}
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := context.WithValue(r.Context(), baseViewModelContextKey, model)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}
