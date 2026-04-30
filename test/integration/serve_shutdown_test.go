//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
)

// TestServe_GracefulShutdown_LetsInFlightActionFinish ensures that when SIGINT
// arrives while an action is mid-flight, Serve waits for the action to finish
// (its output is persisted) before returning.
func TestServe_GracefulShutdown_LetsInFlightActionFinish(t *testing.T) {
	s := startServer(t, WithRun([]string{"sh", "-c", "sleep 1; echo done"}))

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
	pipeID := ids[0]

	// Trigger graceful shutdown while the `sleep 1` action is still running.
	s.shutdown(t)

	select {
	case <-s.Done:
	case <-time.After(10 * time.Second):
		t.Fatalf("Serve did not exit within 10s of SIGINT")
	}

	// Serve has closed its DB handle; reopen and confirm the action ran to
	// completion and its output was flushed.
	db, err := actiondb.New(s.ActionsDB, 0, 0)
	if err != nil {
		t.Fatalf("reopen actions db: %v", err)
	}
	defer db.Close()

	rec, err := db.GetPipelineRecord(pipeID)
	if err != nil {
		t.Fatalf("GetPipelineRecord: %v", err)
	}
	if !rec.EndedAt.Valid {
		t.Fatalf("pipeline %q has no ended_at — Serve exited before action completed", pipeID)
	}
	if rec.Error.Valid {
		t.Errorf("action recorded an error: %q", rec.Error.String)
	}
	if !rec.Output.Valid || !strings.Contains(rec.Output.String, "done") {
		t.Errorf("output should contain 'done', got: %q", rec.Output.String)
	}
}
