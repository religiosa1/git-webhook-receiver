//go:build integration

package integration

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/PuerkitoBio/goquery"
)

// uiPaths are the HTML admin endpoints shared across general auth/disable tests.
var uiPaths = []string{
	"/projects",
	"/pipelines",
	"/logs",
}

func parseHTMLBody(t *testing.T, resp *http.Response) *goquery.Document {
	t.Helper()
	defer resp.Body.Close()
	doc, err := goquery.NewDocumentFromReader(resp.Body)
	if err != nil {
		t.Fatalf("parse HTML: %v", err)
	}
	return doc
}

func assertTestID(t *testing.T, doc *goquery.Document, testID string) {
	t.Helper()
	sel := fmt.Sprintf(`[data-testid="%s"]`, testID)
	if doc.Find(sel).Length() == 0 {
		t.Errorf("expected element with data-testid=%q in document", testID)
	}
}

func TestServe_AdminUI_DisabledReturnsNotFound(t *testing.T) {
	s := startServer(t, WithDisableUI(true))

	cases := []struct {
		path string
		want int
	}{
		{"/projects", http.StatusNotFound},
		{"/pipelines", http.StatusNotFound},
		{"/logs", http.StatusNotFound},
		{"/pipelines/nonexistent", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp := uiGet(t, s.BaseURL, tc.path, nil)
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d (ui disabled)", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestServe_AdminUI_RequiresBasicAuthWhenConfigured(t *testing.T) {
	const user, pass = "admin", "s3cret"
	s := startServer(t, WithBasicAuth(user, pass))

	t.Run("NoCredentials", func(t *testing.T) {
		for _, path := range uiPaths {
			resp := uiGet(t, s.BaseURL, path, nil)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s: status = %d, want 401", path, resp.StatusCode)
			}
		}
	})

	t.Run("WrongCredentials", func(t *testing.T) {
		for _, path := range uiPaths {
			resp := uiGet(t, s.BaseURL, path, &basicCreds{User: user, Pass: "wrong"})
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s: status = %d, want 401", path, resp.StatusCode)
			}
		}
	})

	t.Run("CorrectCredentials", func(t *testing.T) {
		creds := &basicCreds{User: user, Pass: pass}
		for _, path := range uiPaths {
			resp := uiGet(t, s.BaseURL, path, creds)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: status = %d, want 200", path, resp.StatusCode)
			}
		}
	})
}

func TestServe_AdminUI_Projects(t *testing.T) {
	s := startServer(t)

	resp := uiGet(t, s.BaseURL, "/projects", nil)
	if resp.StatusCode != http.StatusOK {
		_, _ = io.Copy(io.Discard, resp.Body)
		resp.Body.Close()
		t.Fatalf("status = %d, want 200", resp.StatusCode)
	}

	doc := parseHTMLBody(t, resp)
	assertTestID(t, doc, "projects-page")
	if !strings.Contains(doc.Text(), defaultProject) {
		t.Errorf("expected project %q in /projects page", defaultProject)
	}
}

func TestServe_AdminUI_Pipelines(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		s := startServer(t)
		resp := uiGet(t, s.BaseURL, "/pipelines", nil)
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "pipelines-empty")
	})

	t.Run("BadRequest_InvalidStatus", func(t *testing.T) {
		s := startServer(t)
		resp := uiGet(t, s.BaseURL, "/pipelines?status=invalid", nil)
		if resp.StatusCode != http.StatusBadRequest {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "bad-request")
	})

	t.Run("WithPipelines", func(t *testing.T) {
		s := startServer(t)
		body, headers := loadGitHubFixture(t)
		headers.Set("X-Hub-Signature-256", signGitHub(defaultSecret, body))
		webhookResp := postWebhook(t, s.BaseURL, defaultProject, headers, body)
		if webhookResp.StatusCode != http.StatusCreated {
			_, _ = io.Copy(io.Discard, webhookResp.Body)
			webhookResp.Body.Close()
			t.Fatalf("webhook status = %d, want 201", webhookResp.StatusCode)
		}
		_, _ = io.Copy(io.Discard, webhookResp.Body)
		webhookResp.Body.Close()

		resp := uiGet(t, s.BaseURL, "/pipelines", nil)
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "pipelines-list")
	})
}

func TestServe_AdminUI_PipelineDetail(t *testing.T) {
	t.Run("OK", func(t *testing.T) {
		s := startServer(t)
		body, headers := loadGitHubFixture(t)
		headers.Set("X-Hub-Signature-256", signGitHub(defaultSecret, body))
		webhookResp := postWebhook(t, s.BaseURL, defaultProject, headers, body)
		if webhookResp.StatusCode != http.StatusCreated {
			_, _ = io.Copy(io.Discard, webhookResp.Body)
			webhookResp.Body.Close()
			t.Fatalf("webhook status = %d, want 201", webhookResp.StatusCode)
		}
		pipeIDs := parsePipeIDs(t, webhookResp)
		webhookResp.Body.Close()
		if len(pipeIDs) == 0 {
			t.Fatal("no pipe IDs returned from webhook")
		}

		resp := uiGet(t, s.BaseURL, "/pipelines/"+pipeIDs[0], nil)
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "pipeline-detail")
		if !strings.Contains(doc.Text(), pipeIDs[0]) {
			t.Errorf("expected pipeId %q in pipeline detail page", pipeIDs[0])
		}
	})

	t.Run("NotFound", func(t *testing.T) {
		s := startServer(t)
		resp := uiGet(t, s.BaseURL, "/pipelines/nonexistent-pipe-id", nil)
		if resp.StatusCode != http.StatusNotFound {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 404", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "not-found")
	})

	t.Run("Auth", func(t *testing.T) {
		const user, pass = "admin", "s3cret"
		s := startServer(t, WithBasicAuth(user, pass))

		t.Run("NoCredentials", func(t *testing.T) {
			resp := uiGet(t, s.BaseURL, "/pipelines/nonexistent-pipe-id", nil)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", resp.StatusCode)
			}
		})

		t.Run("WrongCredentials", func(t *testing.T) {
			resp := uiGet(t, s.BaseURL, "/pipelines/nonexistent-pipe-id", &basicCreds{User: user, Pass: "wrong"})
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("status = %d, want 401", resp.StatusCode)
			}
		})
	})
}

func TestServe_AdminUI_Logs(t *testing.T) {
	t.Run("EmptyList", func(t *testing.T) {
		s := startServer(t)
		resp := uiGet(t, s.BaseURL, "/logs", nil)
		if resp.StatusCode != http.StatusOK {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 200", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "logs-empty")
	})

	t.Run("BadRequest_InvalidLimit", func(t *testing.T) {
		s := startServer(t)
		resp := uiGet(t, s.BaseURL, "/logs?limit=-1", nil)
		if resp.StatusCode != http.StatusBadRequest {
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			t.Fatalf("status = %d, want 400", resp.StatusCode)
		}
		doc := parseHTMLBody(t, resp)
		assertTestID(t, doc, "bad-request")
	})
}
