//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
)

func waitForOutput(t *testing.T, dbPath, pipeID string, timeout time.Duration) string {
	t.Helper()
	db, err := actionsdb.New(dbPath, 0)
	if err != nil {
		t.Fatalf("open actions db: %v", err)
	}
	defer db.Close()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		output, err := db.GetPipelineOutput(pipeID)
		if err == nil {
			return string(output)
		}
		time.Sleep(50 * time.Millisecond)
	}
	t.Fatalf("pipeline %q did not finish within %s", pipeID, timeout)
	return ""
}

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

	output := waitForOutput(t, s.ActionsDB, ids[0], 10*time.Second)
	if strings.Contains(output, "PATH=") {
		t.Errorf("output should contain PATH= (env stdout), got: %q", output)
	}
}
