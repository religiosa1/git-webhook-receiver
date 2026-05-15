package views_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/views"
)

func getBaseViewModelContext(t *testing.T, publicURL, requestPath string) context.Context {
	t.Helper()
	var ctx context.Context

	handler := views.WithBaseViewModel(config.Config{
		PublicURL: publicURL,
	})(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		ctx = r.Context()
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, requestPath, nil)
	rr := httptest.NewRecorder()
	handler.ServeHTTP(rr, req)

	return ctx
}

func TestMakePublicUrl(t *testing.T) {
	tests := []struct {
		name      string
		publicURL string
		want      string
	}{
		{"returns full url if that's what's supplied", "https://example.com/", "https://example.com/foo"},
		{"merges full url path", "https://example.com/bar/", "https://example.com/bar/foo"},
		{"merges full url path adding slash", "https://example.com/bar", "https://example.com/bar/foo"},
		{"ignores missing trailing slash in url", "https://example.com", "https://example.com/foo"},
		{"returns relative url if nothing is supplied", "", "/foo"},
		{"merges on top of whatever provided otherwise", "qwerty", "qwerty/foo"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := getBaseViewModelContext(t, tt.publicURL, "/")
			got := views.MakePublicURL(ctx, "foo")
			if got != tt.want {
				t.Errorf("Unexpected PublicURL value, want %q, got %q", tt.want, got)
			}
		})
	}
}

func TestCurrentPath(t *testing.T) {
	tests := []struct {
		name        string
		publicURL   string
		requestPath string
		want        string
	}{
		{"simple case", "https://example.com/", "/foo", "/foo"},
		{"strips query params", "https://example.com/", "/foo?bar=biz#baz", "/foo"},
		{"strips relative path suffix", "https://example.com/bar", "/bar/foo", "/foo"},
		{"strips relative path suffix with a trailing /", "https://example.com/bar/", "/bar/foo", "/foo"},
		{"falls back to full path, if there's nothing to strip", "https://example.com/bar/", "/biz/foo", "/biz/foo"},
		{"total strip still returns /", "http://example.com/foo", "/foo", "/"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := getBaseViewModelContext(t, tt.publicURL, tt.requestPath)
			got := views.GetBaseViewModel(ctx).CurrentPath
			if got != tt.want {
				t.Errorf("Unexpected CurrentPath value, want %q, got %q", tt.want, got)
			}
		})
	}
}
