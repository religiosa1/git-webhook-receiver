//go:build integration

package integration

import (
	"io"
	"net/http"
	"strings"
	"testing"
	"time"
)

// Authorization (the project-level token sent in the Authorization header) is
// only supported by Gitea/GitLab; GitHub doesn't use it. Covered here on Gitea.
func TestServe_Gitea_AuthorizationHeader(t *testing.T) {
	const authToken = "myauthtoken"
	s := startServer(t,
		WithProvider("gitea"),
		WithAuthorization(authToken),
	)

	body, headers := loadGiteaFixture(t)
	headers.Set("X-Gitea-Signature", signGitea(defaultSecret, body))

	t.Run("CorrectAuthorization", func(t *testing.T) {
		h := headers.Clone()
		h.Set("Authorization", authToken)

		resp := postWebhook(t, s.BaseURL, defaultProject, h, body)
		defer resp.Body.Close()
		if resp.StatusCode != http.StatusCreated {
			b, _ := io.ReadAll(resp.Body)
			t.Fatalf("status = %d, want 201; body=%s", resp.StatusCode, b)
		}

		ids := parsePipeIDs(t, resp)
		if len(ids) != 1 {
			t.Fatalf("got %d pipe ids, want 1: %v", len(ids), ids)
		}
		rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
		if rec.Error.Valid {
			t.Errorf("action recorded an error: %q", rec.Error.String)
		}
		if !rec.Output.Valid || !strings.Contains(rec.Output.String, "PATH=") {
			t.Errorf("output should contain PATH=, got: %q", rec.Output.String)
		}
	})

	t.Run("WrongAuthorization", func(t *testing.T) {
		h := headers.Clone()
		h.Set("Authorization", "wrong-token")

		resp := postWebhook(t, s.BaseURL, defaultProject, h, body)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})

	t.Run("MissingAuthorization", func(t *testing.T) {
		h := headers.Clone()
		h.Del("Authorization")

		resp := postWebhook(t, s.BaseURL, defaultProject, h, body)
		defer resp.Body.Close()
		_, _ = io.Copy(io.Discard, resp.Body)
		if resp.StatusCode != http.StatusUnauthorized {
			t.Fatalf("status = %d, want 401", resp.StatusCode)
		}
	})
}
