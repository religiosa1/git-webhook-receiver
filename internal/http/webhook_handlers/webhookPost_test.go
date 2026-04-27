package webhookhandlers_test

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/ActionRunner"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	handlers "github.com/religiosa1/git-webhook-receiver/internal/http/webhook_handlers"
	"github.com/religiosa1/git-webhook-receiver/internal/requestmock"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

type ResponseStats struct {
	Ok           int
	Forbidden    int
	BadSignature int
}

func makeActionsList(actions ...config.Action) []config.Action {
	defaultAction := config.Action{
		On:     "push",
		Branch: "master",
		Run:    []string{"go", "version"},
	}

	mergedActions := make([]config.Action, len(actions))
	for i, a := range actions {
		mergedAction := a
		if mergedAction.On == "" {
			mergedAction.On = defaultAction.On
		}
		if mergedAction.Branch == "" {
			mergedAction.Branch = defaultAction.Branch
		}
		if mergedAction.Run == nil {
			mergedAction.Run = defaultAction.Run
		}
		mergedActions[i] = mergedAction
	}
	return mergedActions
}

var (
	authToken       = "JgHhtuPOISmw3WDCRtz4H6IrT8zWwNkS"
	secret          = "cc7ec03e-2e09-4bb9-b2fc-388b865200d0"
	projectName     = "testProj"
	projectEndPoint = fmt.Sprintf("/project/%s", url.PathEscape(projectName))
)

func loadMockRequest(t *testing.T) requestmock.RequestMock {
	return requestmock.LoadRequestMock(t, "../../requestmock/captured-requests/gitea.json")
}

type testHandler struct {
	handlers.Webhook
}

func newTestHandler(cfg config.Config, prj config.Project) testHandler {
	rcvr := whreceiver.New(prj)
	h := handlers.Webhook{
		ActionsCh:   make(chan ActionRunner.ActionArgs, 10),
		Config:      cfg,
		ProjectName: projectName,
		Project:     prj,
		Receiver:    rcvr,
	}
	return testHandler{h}
}

func (h testHandler) doRequestAndGetAction(t *testing.T, req *http.Request) handlers.ActionOutput {
	t.Helper()
	response := httptest.NewRecorder()
	h.ServeHTTP(response, req)

	result := response.Result()
	if result.StatusCode != 201 {
		body, _ := io.ReadAll(result.Body)
		t.Fatalf("Expected status 201 OK, got %v, %s", result.StatusCode, string(body))
	}

	actions := make([]handlers.ActionOutput, 0)
	if err := json.NewDecoder(result.Body).Decode(&actions); err != nil {
		t.Fatal(err)
	}

	if l := len(actions); l != 1 {
		t.Fatalf("unexpected length of actions in the response, want 1, got %d", l)
	}
	return actions[0]
}

func TestProjectMatching(t *testing.T) {
	requestDump := loadMockRequest(t)

	cfg := config.Config{}
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions:     makeActionsList(config.Action{}),
	}

	t.Run("returns 201 if some of the actions matches", func(t *testing.T) {
		request := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		newTestHandler(cfg, prj).ServeHTTP(response, request)
		got := response.Result().StatusCode
		want := 201

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("returns 204, if no action matches", func(t *testing.T) {
		prj2 := prj
		prj2.Actions = makeActionsList(config.Action{Branch: "badbranch"})

		request := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		newTestHandler(cfg, prj2).ServeHTTP(response, request)

		got := response.Result().StatusCode
		want := 204

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}

		body := response.Body.String()
		if body != "" {
			t.Errorf("Expect body to be empty, got %s", body)
		}
	})

	secretAndAuthStatusTests := []struct {
		name       string
		auth       string
		secret     string
		wantStatus int
	}{
		{"correct signature", "", secret, 201},
		{"bad signature", "", "bad key", 403},
		{"correct auth", authToken, "", 201},
		{"bad auth", "bad pass", "", 401},
		{"bad auth precedes bad sign", "bad pass", "bad key", 401},
	}

	for _, tt := range secretAndAuthStatusTests {
		t.Run(tt.name, func(t *testing.T) {
			actn := config.Action{}

			prj2 := prj
			prj2.Authorization = tt.auth
			prj2.Secret = tt.secret
			prj2.Actions = makeActionsList(actn)

			request := requestDump.ToHttpRequest(projectEndPoint)
			response := httptest.NewRecorder()

			newTestHandler(cfg, prj2).ServeHTTP(response, request)

			gotStatus := response.Result().StatusCode

			if gotStatus != tt.wantStatus {
				t.Errorf("got status %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}

func TestActionMatching(t *testing.T) {
	requestDump := loadMockRequest(t)

	cfg := config.Config{}
	baseProject := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
	}

	runHandler := func(t *testing.T, actions []config.Action) int {
		t.Helper()
		request := requestDump.ToHttpRequest(projectEndPoint) // branch: "master", event: "push"
		prj := baseProject
		prj.Actions = actions
		response := httptest.NewRecorder()
		newTestHandler(cfg, prj).ServeHTTP(response, request)
		return response.Result().StatusCode
	}

	t.Run("branch matching", func(t *testing.T) {
		cases := []struct {
			name       string
			branch     string
			wantStatus int
		}{
			{"exact match", "master", 201},
			{"no match", "other", 204},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				got := runHandler(t, []config.Action{{On: "push", Branch: tt.branch, Run: []string{"go", "version"}}})
				if got != tt.wantStatus {
					t.Errorf("got %d, want %d", got, tt.wantStatus)
				}
			})
		}
	})

	t.Run("branch wildcard matches any branch", func(t *testing.T) {
		got := runHandler(t, []config.Action{{On: "push", Branch: "*", Run: []string{"go", "version"}}})
		if got != 201 {
			t.Errorf("got %d, want 201", got)
		}
	})

	t.Run("event matching", func(t *testing.T) {
		cases := []struct {
			name       string
			event      string
			wantStatus int
		}{
			{"exact match", "push", 201},
			{"no match", "release", 204},
		}
		for _, tt := range cases {
			t.Run(tt.name, func(t *testing.T) {
				got := runHandler(t, []config.Action{{On: tt.event, Branch: "master", Run: []string{"go", "version"}}})
				if got != tt.wantStatus {
					t.Errorf("got %d, want %d", got, tt.wantStatus)
				}
			})
		}
	})

	t.Run("event wildcard matches any event", func(t *testing.T) {
		got := runHandler(t, []config.Action{{On: "*", Branch: "master", Run: []string{"go", "version"}}})
		if got != 201 {
			t.Errorf("got %d, want 201", got)
		}
	})
}

func TestResponseBody(t *testing.T) {
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions: []config.Action{
			{
				On:     "push",
				Branch: "non-existing",
				Run:    []string{"go", "version"},
			},
			{
				On:     "push",
				Branch: "master",
				Run:    []string{"go", "version"},
			},
		},
	}
	request := loadMockRequest(t).ToHttpRequest(projectEndPoint)
	handler := newTestHandler(config.Config{}, prj)

	t.Run("contains action identifier of matched action", func(t *testing.T) {
		action := handler.doRequestAndGetAction(t, request)

		if want, got := 1, action.Index; want != got {
			t.Errorf("Unexpected action index value, want %d, got %d", want, got)
		}

		if want, got := projectName, action.Project; want != got {
			t.Errorf("Unexpected project name value, got %s, want %s", want, got)
		}

		if action.PipeID == "" {
			t.Errorf("Unexpected empty PipeID: %v", action)
		}
	})
}

func TestPublicUrl(t *testing.T) {
	requestDump := loadMockRequest(t)
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions: []config.Action{
			{
				On:     "push",
				Branch: "master",
				Run:    []string{"go", "version"},
			},
		},
	}

	t.Run("returns a list of links, if publicURL is present in config", func(t *testing.T) {
		publicURL := "ftp://example.com/"
		cfg := config.Config{PublicURL: publicURL}
		action := newTestHandler(cfg, prj).doRequestAndGetAction(t, requestDump.ToHttpRequest(projectEndPoint))
		want := publicURL + "pipelines/" + url.PathEscape(action.PipeID)
		if action.Links == nil {
			t.Fatal("unexpected empty links object")
		}
		if got := action.Links.Details; want != got {
			t.Errorf("details link is wrong, want %q. got %q", want, got)
		}
		want = want + "/output"
		if got := action.Links.Output; want != got {
			t.Errorf("output link is wrong, want %q. got %q", want, got)
		}
	})

	t.Run("trailing slash is optional for the public url", func(t *testing.T) {
		publicURL := "ftp://example.com"
		cfg := config.Config{PublicURL: publicURL}
		action := newTestHandler(cfg, prj).doRequestAndGetAction(t, requestDump.ToHttpRequest(projectEndPoint))
		want := publicURL + "/pipelines/" + url.PathEscape(action.PipeID)
		if action.Links == nil {
			t.Fatal("unexpected empty links object")
		}
		if got := action.Links.Details; want != got {
			t.Errorf("details link is wrong, want %q. got %q", want, got)
		}
	})

	t.Run("no links field is present if inspection API is disabled", func(t *testing.T) {
		publicURL := "ftp://example.com"
		cfg := config.Config{PublicURL: publicURL, DisableAPI: true}
		action := newTestHandler(cfg, prj).doRequestAndGetAction(t, requestDump.ToHttpRequest(projectEndPoint))
		if action.Links != nil {
			t.Fatalf("Expected to get empty links, got %v", action.Links)
		}
	})

	t.Run("no links field is present if publicURL is not configured", func(t *testing.T) {
		cfg := config.Config{}
		action := newTestHandler(cfg, prj).doRequestAndGetAction(t, requestDump.ToHttpRequest(projectEndPoint))
		if action.Links != nil {
			t.Fatalf("Expected to get empty links, got %v", action.Links)
		}
	})
}
