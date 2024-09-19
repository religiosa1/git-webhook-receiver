package webhookhandlers_test

import (
	"io"
	"log/slog"
	"net/http/httptest"
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

func TestProjectMatching(t *testing.T) {
	ch := make(chan ActionRunner.ActionArgs)

	go func() {
		for range ch {
			// Do nothing, just drain the channel
		}
	}()

	requestDump := requestmock.LoadRequestMock(t, "../../requestmock/captured-requests/gitea.json")
	authToken := "JgHhtuPOISmw3WDCRtz4H6IrT8zWwNkS"
	secret := "cc7ec03e-2e09-4bb9-b2fc-388b865200d0"

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	cfg := config.Config{}
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions:     makeActionsList(config.Action{}),
	}
	rcvr := whreceiver.New(prj)

	projectName := "testProj"
	projectEndPoint := "/" + projectName

	t.Run("returns 201 if some of the actions matches", func(t *testing.T) {
		request := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		handlers.HandleWebhookPost(ch, logger, cfg, projectName, prj, rcvr)(response, request)

		got := response.Result().StatusCode
		want := 201

		if got != want {
			t.Errorf("got %d, want %d", got, want)
		}
	})

	t.Run("returns 200, if no action matches", func(t *testing.T) {
		prj2 := prj
		prj2.Actions = makeActionsList(config.Action{Branch: "badbranch"})

		request := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()

		handlers.HandleWebhookPost(ch, logger, cfg, projectName, prj2, rcvr)(response, request)

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

			handlers.HandleWebhookPost(ch, logger, cfg, projectName, prj2, rcvr)(response, request)

			gotStatus := response.Result().StatusCode

			if gotStatus != tt.wantStatus {
				t.Errorf("got status %d, want %d", gotStatus, tt.wantStatus)
			}
		})
	}
}

// TODO response body test cases
