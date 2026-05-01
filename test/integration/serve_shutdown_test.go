//go:build integration

package integration

import (
	"bytes"
	"net/http"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
)

// TestServe_GracefulShutdown_LetsInFlightActionFinish ensures that when SIGINT
// arrives while an action is mid-flight, Serve waits for the action to finish
// (its output is persisted) before returning.
func TestServe_GracefulShutdown_LetsInFlightActionFinish(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{"script", WithScript("sleep 1\necho done")},
		{"run", WithRun([]string{"sh", "testdata/finish_action.sh"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := startServer(t, tt.opt)

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

			// Trigger graceful shutdown while the action is still running.
			s.shutdown(t)

			// Wait for srv.Shutdown() to close the listener (signal → cancel →
			// Shutdown is near-instant; 200 ms is well within the 1-second window).
			time.Sleep(200 * time.Millisecond)

			// Server must not accept new connections during graceful shutdown.
			body2, headers2 := loadGitHubFixture(t)
			headers2.Set("X-Hub-Signature-256", signGitHub(defaultSecret, body2))
			req2, err := http.NewRequest(http.MethodPost, s.BaseURL+"/projects/"+defaultProject, bytes.NewReader(body2))
			if err != nil {
				t.Fatalf("build second request: %v", err)
			}
			for k, vv := range headers2 {
				for _, v := range vv {
					req2.Header.Add(k, v)
				}
			}
			if resp2, err2 := http.DefaultClient.Do(req2); err2 == nil {
				resp2.Body.Close()
				t.Error("server accepted new request during graceful shutdown, want connection refused")
			}

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
		})
	}
}

// TestServe_GracefulShutdown_SecondSIGINTKillsActions ensures that a second
// SIGINT received during graceful shutdown cancels in-flight actions immediately
// instead of waiting for them to finish.
func TestServe_GracefulShutdown_SecondSIGINTKillsActions(t *testing.T) {
	tests := []struct {
		name string
		opt  Option
	}{
		{"script", WithScript("sleep 10\necho done")},
		{"run", WithRun([]string{"sh", "testdata/kill_action.sh"})},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			s := startServer(t, tt.opt)

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

			// First SIGINT: begin graceful shutdown.
			s.shutdown(t)

			// Wait for the HTTP server to close its listener before sending the
			// second signal (signal → cancel → srv.Shutdown is near-instant;
			// 200 ms is well within the 10-second sleep window).
			time.Sleep(200 * time.Millisecond)

			// Second SIGINT: force-cancel in-flight actions.
			if err := syscall.Kill(os.Getpid(), syscall.SIGINT); err != nil {
				t.Fatalf("send second SIGINT: %v", err)
			}

			// Server must exit well before the 10-second sleep would complete.
			select {
			case <-s.Done:
			case <-time.After(5 * time.Second):
				t.Fatalf("Serve did not exit within 5s after second SIGINT")
			}

			// Reopen DB and confirm the action was killed before it could print "done".
			db, err := actiondb.New(s.ActionsDB, 0, 0)
			if err != nil {
				t.Fatalf("reopen actions db: %v", err)
			}
			defer db.Close()

			rec, err := db.GetPipelineRecord(pipeID)
			if err != nil {
				t.Fatalf("GetPipelineRecord: %v", err)
			}
			if rec.Output.Valid && strings.Contains(rec.Output.String, "done") {
				t.Errorf("action should have been killed before completion, but output contains 'done': %q", rec.Output.String)
			}
		})
	}
}
