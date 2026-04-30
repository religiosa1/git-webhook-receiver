//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"
)

func TestServe_GitHubPush_RunsActionAndPersistsOutput(t *testing.T) {
	s := startServer(t)

	body, headers := loadGitHubFixture(t)
	headers.Set("X-Hub-Signature-256", signGitHub(defaultSecret, body))

	resp := postWebhook(t, s.BaseURL, defaultProject, headers, body)
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("status = %d, want 201", resp.StatusCode)
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
		t.Errorf("output should contain PATH= (env stdout), got: %q", rec.Output.String)
	}
	if rec.Project != defaultProject {
		t.Errorf("project = %q, want %q", rec.Project, defaultProject)
	}
}
