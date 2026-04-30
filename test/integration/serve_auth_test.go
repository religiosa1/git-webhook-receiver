//go:build integration

package integration

import (
	"io"
	"net/http"
	"testing"
)

func TestServe_RejectsBadSignature(t *testing.T) {
	s := startServer(t)
	body, headers := loadGitHubFixture(t)

	t.Run("BadSignature", func(t *testing.T) {
		h := headers.Clone()
		h.Set("X-Hub-Signature-256", signGitHub("wrongsecret", body))

		resp := postWebhook(t, s.BaseURL, defaultProject, h, body)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
	})

	t.Run("MissingSignature", func(t *testing.T) {
		h := headers.Clone()
		h.Del("X-Hub-Signature-256")
		h.Del("X-Hub-Signature")

		resp := postWebhook(t, s.BaseURL, defaultProject, h, body)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusForbidden {
			t.Fatalf("status = %d, want 403", resp.StatusCode)
		}
	})
}
