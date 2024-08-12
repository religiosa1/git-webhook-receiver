package handlers_test

import (
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

	go func() {
		for range ch {
			// Do nothing, just drain the channel
		}
	}()

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

	t.Run("returns 201 if some of the actions matches", func(t *testing.T) {
		request, _ := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		handlers.HandleWebhookPost(ch, logger, cfg, prj, rcvr)(response, request)

		got := response.Result().StatusCode
		want := 201

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("returns 200, if no action matches", func(t *testing.T) {
		prj2 := prj
		prj2.Actions = makeActionsList(config.Action{Branch: "badbranch"})

		request, _ := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()

		handlers.HandleWebhookPost(ch, logger, cfg, prj2, rcvr)(response, request)

		got := response.Result().StatusCode
		want := 200

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
		{"correct signature", "", requestDump.Secret, 201},
		{"bad signature", "", "bad key", 403},
		{"correct auth", "123456", "", 201},
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

			request, _ := requestDump.ToHttpRequest(projectEndPoint)
			response := httptest.NewRecorder()

			handlers.HandleWebhookPost(ch, logger, cfg, prj2, rcvr)(response, request)

			gotStatus := response.Result().StatusCode

			if gotStatus != tt.wantStatus {
				t.Errorf("got status %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}

// TODO response body test cases
