package handlers_test

import (
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/action_runner"
	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/http/handlers"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

type ResponseStats struct {
	Ok           int
	Forbidden    int
	BadSignature int
}

func responseStatsFrom(res *handlers.WebhookPostResult) ResponseStats {
	return ResponseStats{
		Ok:           len(res.Ok),
		Forbidden:    len(res.Forbidden),
		BadSignature: len(res.BadSignature),
	}
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

func TestProjectMatching(t *testing.T) {
	ch := make(chan action_runner.ActionArgs)
	requestDump, err := LoadRequestMock("./requests_test/gitea.json")
	if err != nil {
		log.Fatalf("Unable to load request dump: %e", err)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := &config.Config{}
	prj := &config.Project{
		GitProvider: "gitea",
		Repo:        "baruser/foorepo",
		Actions:     makeActionsList(config.Action{}),
	}
	rcvr := whreceiver.New(prj)

	projectEndPoint := "/testProj"

	t.Run("returns a list of successfull actions", func(t *testing.T) {
		request, _ := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		handlers.HandleWebhookPost(ch, logger, cfg, prj, rcvr)(response, request)

		got := response.Result().StatusCode
		want := 200

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("returns 204, if no actions match", func(t *testing.T) {
		prj2 := prj
		prj2.Actions = makeActionsList(config.Action{Branch: "badbranch"})

		request, _ := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()

		handlers.HandleWebhookPost(ch, logger, cfg, prj2, rcvr)(response, request)

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

	secretAndAuthTests := []struct {
		name       string
		auth       string
		secret     string
		wantStatus int
		wantResp   ResponseStats
	}{
		{"correct signature", "", requestDump.Secret, 200, ResponseStats{1, 0, 0}},
		{"bad signature", "", "bad key", 403, ResponseStats{0, 0, 1}},
		{"correct auth", "123456", "", 200, ResponseStats{1, 0, 0}},
		{"bad auth", "bad pass", "", 403, ResponseStats{0, 1, 0}},
		{"bad auth precedes bad sign", "bad pass", "bad key", 403, ResponseStats{0, 1, 0}},
	}

	for _, tt := range secretAndAuthTests {
		t.Run(tt.name, func(t *testing.T) {
			actn := config.Action{}
			if tt.auth != "" {
				actn.Authorization = tt.auth
			}
			if tt.secret != "" {
				actn.Secret = tt.secret
			}

			prj2 := prj
			prj2.Actions = makeActionsList(actn)

			request, _ := requestDump.ToHttpRequest(projectEndPoint)
			response := httptest.NewRecorder()

			handlers.HandleWebhookPost(ch, logger, cfg, prj2, rcvr)(response, request)

			gotStatus := response.Result().StatusCode

			if gotStatus != tt.wantStatus {
				t.Errorf("got status %d, want %d", gotStatus, tt.wantStatus)
			}

			var body handlers.WebhookPostResult
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Errorf("Unable to retrieve response body json: %s", err)
			}
			got := responseStatsFrom(&body)

			if got != tt.wantResp {
				t.Errorf("Unexpected response, got %v, want %v", got, tt.wantResp)
			}
		})
	}
}
