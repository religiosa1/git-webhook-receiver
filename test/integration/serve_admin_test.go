//go:build integration

package integration

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"
)

// adminPaths are the read-side admin endpoints that should follow the
// disable_api / basic-auth gating. Listed once and reused by every admin test.
var adminPaths = []string{
	"/api/projects",
	"/api/projects/" + defaultProject,
	"/api/pipelines",
	"/api/logs",
}

func TestServe_Admin_DisabledReturnsUnreachable(t *testing.T) {
	s := startServer(t, WithDisableAPI(true))

	cases := []struct {
		path string
		want int
	}{
		{"/api/projects", http.StatusNotFound},
		{"/api/projects/" + defaultProject, http.StatusNotFound},
		{"/api/pipelines", http.StatusNotFound},
		{"/api/logs", http.StatusNotFound},
	}

	for _, tc := range cases {
		t.Run(tc.path, func(t *testing.T) {
			resp := adminGet(t, s.BaseURL, tc.path, nil)
			defer resp.Body.Close()
			_, _ = io.Copy(io.Discard, resp.Body)
			if resp.StatusCode != tc.want {
				t.Fatalf("status = %d, want %d (admin disabled)", resp.StatusCode, tc.want)
			}
		})
	}
}

func TestServe_Admin_RequiresBasicAuthWhenConfigured(t *testing.T) {
	const user, pass = "admin", "s3cret"
	s := startServer(t,
		WithDisableAPI(false),
		WithBasicAuth(user, pass),
	)

	t.Run("NoCredentials", func(t *testing.T) {
		for _, path := range adminPaths {
			resp := adminGet(t, s.BaseURL, path, nil)
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s: status = %d, want 401", path, resp.StatusCode)
			}
		}
	})

	t.Run("WrongCredentials", func(t *testing.T) {
		for _, path := range adminPaths {
			resp := adminGet(t, s.BaseURL, path, &basicCreds{User: user, Pass: "wrong"})
			_, _ = io.Copy(io.Discard, resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusUnauthorized {
				t.Errorf("%s: status = %d, want 401", path, resp.StatusCode)
			}
		}
	})

	t.Run("CorrectCredentials", func(t *testing.T) {
		creds := &basicCreds{User: user, Pass: pass}
		for _, path := range adminPaths {
			resp := adminGet(t, s.BaseURL, path, creds)
			body, _ := io.ReadAll(resp.Body)
			resp.Body.Close()
			if resp.StatusCode != http.StatusOK {
				t.Errorf("%s: status = %d, want 200; body=%s", path, resp.StatusCode, body)
				continue
			}
			if got := resp.Header.Get("Content-Type"); got != "application/json" {
				t.Errorf("%s: response Content-Type = %q, want application/json", path, got)
			}
			// Sanity-check the payload is valid JSON.
			var any interface{}
			if err := json.Unmarshal(body, &any); err != nil {
				t.Errorf("%s: response is not valid JSON: %v; body=%s", path, err, body)
			}
		}
	})

	t.Run("ListProjectsContainsTestProject", func(t *testing.T) {
		resp := adminGet(t, s.BaseURL, "/api/projects", &basicCreds{User: user, Pass: pass})
		defer resp.Body.Close()
		var projects map[string]interface{}
		if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
			t.Fatalf("decode: %v", err)
		}
		if _, ok := projects[defaultProject]; !ok {
			t.Errorf("project %q missing from /api/projects response: %v", defaultProject, projects)
		}
	})
}

// When disable_api is false but no api_user / api_password are configured the
// basic-auth middleware turns into a no-op, so admin endpoints become
// accessible without credentials. Pinning that current behavior here.
func TestServe_Admin_NoCredsConfiguredAllowsAccess(t *testing.T) {
	s := startServer(t, WithDisableAPI(false))

	resp := adminGet(t, s.BaseURL, "/api/projects", nil)
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d, want 200 (no creds configured = open)", resp.StatusCode)
	}
}
