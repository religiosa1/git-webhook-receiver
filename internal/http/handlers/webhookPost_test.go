package handlers_test

import (
	"encoding/json"
	"io"
	"log"
	"log/slog"
	"net/http/httptest"
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/http/handlers"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

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

		handlers.HandleWebhookPost(logger, cfg, prj, rcvr)(response, request)

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

		handlers.HandleWebhookPost(logger, cfg, prj2, rcvr)(response, request)

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

	authTests := []struct {
		name          string
		auth          string
		wantStatus    int
		wantOk        int
		wantForbidden int
	}{
		{"correct auth", "123456", 200, 1, 0},
		{"bad auth", "bad pass", 403, 0, 1},
	}

	for _, tt := range authTests {
		t.Run(tt.name, func(t *testing.T) {
			prj2 := prj
			prj2.Actions = makeActionsList(config.Action{Authorization: tt.auth})

			request, _ := requestDump.ToHttpRequest(projectEndPoint)
			response := httptest.NewRecorder()

			handlers.HandleWebhookPost(logger, cfg, prj2, rcvr)(response, request)

			gotStatus := response.Result().StatusCode

			if gotStatus != tt.wantStatus {
				t.Errorf("got status %d, want %d", gotStatus, tt.wantStatus)
			}

			var body handlers.WebhookPostResult
			if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
				t.Errorf("Unable to retrieve response body json %e", err)
			}

			if got := len(body.BadSignature); got != 0 {
				t.Errorf("Bad number of BadSignature items in response, got %d, want %d", got, 0)
			}
			if got := len(body.Ok); got != tt.wantOk {
				t.Errorf("Bad number of Ok items in response, got %d, want %d", got, tt.wantOk)
			}
			if got := len(body.Forbidden); got != tt.wantForbidden {
				t.Errorf("Bad number of Ok items in response, got %d, want %d", got, tt.wantForbidden)
			}
		})
	}
}
