//go:build integration

package integration

import (
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
)

const errOutputTooLarge = "output buffer exceeded maximum size"

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

func postGitHubWebhook(t *testing.T, s *testServer) []string {
	t.Helper()
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
	return ids
}

func getPipelineOutput(t *testing.T, dbPath, pipeID string) string {
	t.Helper()
	db, err := actionsdb.New(dbPath, 0)
	if err != nil {
		t.Fatalf("open actions db: %v", err)
	}
	defer db.Close()
	output, err := db.GetPipelineOutput(pipeID)
	if err != nil {
		t.Fatalf("GetPipelineOutput(%q): %v", pipeID, err)
	}
	return string(output)
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

func TestServe_GitHubPush_OutputWithinLimit(t *testing.T) {
	const (
		maxOutputBytes = 50
		script         = "printf 'hello'"
		wantOutput     = "hello"
	)

	s := startServer(t, WithMaxOutputBytes(maxOutputBytes), WithScript(script))
	ids := postGitHubWebhook(t, s)

	rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
	if rec.Error != nil {
		t.Fatalf("pipeline failed unexpectedly: %s", rec.Error)
	}

	got := getPipelineOutput(t, s.ActionsDB, ids[0])
	if got != wantOutput {
		t.Errorf("output = %q, want %q", got, wantOutput)
	}
}

func TestServe_GitHubPush_ActionFails(t *testing.T) {
	t.Run("exit1 records error", func(t *testing.T) {
		s := startServer(t, WithScript("exit 1"))
		ids := postGitHubWebhook(t, s)
		rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
		if rec.Error == nil {
			t.Fatal("want error recorded, got none")
		}
		if !strings.Contains(rec.Error.Error(), "exit status 1") {
			t.Errorf("error = %q, want it to contain %q", rec.Error, "exit status 1")
		}
	})

	// The sh interpreter writes "command not found" to stderr (merged into output),
	// and records exit status 127 as the error — not the "not found" message itself.
	t.Run("bad command records exit127 and captures not-found message in output", func(t *testing.T) {
		s := startServer(t, WithScript("nonexistent_command_xyz"))
		ids := postGitHubWebhook(t, s)
		rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
		if rec.Error == nil {
			t.Fatal("want error recorded, got none")
		}
		if !strings.Contains(rec.Error.Error(), "exit status 127") {
			t.Errorf("error = %q, want it to contain %q", rec.Error, "exit status 127")
		}
		got := getPipelineOutput(t, s.ActionsDB, ids[0])
		if !strings.Contains(got, "nonexistent_command_xyz") {
			t.Errorf("output = %q, want it to mention the bad command name", got)
		}
	})

	t.Run("stderr is captured in output", func(t *testing.T) {
		s := startServer(t, WithScript("echo 'oops' >&2; exit 1"))
		ids := postGitHubWebhook(t, s)
		rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
		if rec.Error == nil {
			t.Fatal("want error recorded, got none")
		}
		got := getPipelineOutput(t, s.ActionsDB, ids[0])
		if !strings.Contains(got, "oops") {
			t.Errorf("output = %q, want it to contain stderr text %q", got, "oops")
		}
	})
}

func TestServe_GitHubPush_OutputTooLarge(t *testing.T) {
	const (
		maxOutputBytes = 50
		// 70 'a's, exceeds the 50-byte limit
		bigOutput = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
		script    = "printf '" + bigOutput + "'"
	)

	s := startServer(t, WithMaxOutputBytes(maxOutputBytes), WithScript(script))
	ids := postGitHubWebhook(t, s)

	rec := waitForPipeline(t, s.ActionsDB, ids[0], 10*time.Second)
	if rec.Error == nil {
		t.Fatal("want pipeline to fail with output-too-large error, but no error recorded")
	}
	if !strings.Contains(rec.Error.Error(), errOutputTooLarge) {
		t.Errorf("error = %q, want it to contain %q", rec.Error, errOutputTooLarge)
	}

	got := getPipelineOutput(t, s.ActionsDB, ids[0])
	if len(got) == 0 {
		t.Fatal("want truncated output to be persisted, got empty output")
	}
	if len(got) > maxOutputBytes {
		t.Errorf("output length %d exceeds max %d", len(got), maxOutputBytes)
	}
	if !strings.HasPrefix(bigOutput, got) {
		t.Errorf("output %q is not a prefix of expected %q", got, bigOutput)
	}
}
