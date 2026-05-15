package views

import (
	"context"
	"net/http"
	"net/url"
	"strings"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type baseViewModel struct {
	HasLogsPages     bool
	HasPipelinePages bool
	PublicURL        string
	CurrentPath      string
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
	publicURL := GetBaseViewModel(ctx).PublicURL
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
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			currentPath := cutPublicPathPrefix(cfg.PublicURL, r.URL.Path)
			model := baseViewModel{
				HasLogsPages:     cfg.LogsDBFile != "",
				HasPipelinePages: cfg.ActionsDBFile != "",
				PublicURL:        cfg.PublicURL,
				CurrentPath:      currentPath,
			}
			ctx := context.WithValue(r.Context(), baseViewModelContextKey, model)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

func cutPublicPathPrefix(publicURL, currentPath string) string {
	if publicURL == "" {
		return currentPath
	}
	url, err := url.Parse(publicURL)
	if err != nil {
		return currentPath
	}
	if url.Path == "" || url.Path == "/" {
		return currentPath
	}
	path, _ := strings.CutSuffix(url.Path, "/")
	stripped, _ := strings.CutPrefix(currentPath, path)
	if stripped == "" {
		stripped = "/"
	}

	return stripped
}
