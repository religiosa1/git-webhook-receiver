package webhookhandlers_test

import (
	"encoding/json"
	"io"
	"log/slog"
	"net/http/httptest"
	"strings"
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

var authToken = "JgHhtuPOISmw3WDCRtz4H6IrT8zWwNkS"
var secret = "cc7ec03e-2e09-4bb9-b2fc-388b865200d0"
var projectName = "testProj"
var projectEndPoint = "/" + projectName

func loadMockRequest(t *testing.T) requestmock.RequestMock {
	return requestmock.LoadRequestMock(t, "../../requestmock/captured-requests/gitea.json")
}

func makeChannelDrainer() chan ActionRunner.ActionArgs {
	ch := make(chan ActionRunner.ActionArgs)

	go func() {
		for range ch {
			// Do nothing, just drain the channel
		}
	}()

	return ch
}

func TestProjectMatching(t *testing.T) {
	ch := makeChannelDrainer()
	var requestDump = loadMockRequest(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := config.Config{}
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions:     makeActionsList(config.Action{}),
	}
	rcvr := whreceiver.New(prj)

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

func TestResponseBody(t *testing.T) {
	ch := makeChannelDrainer()
	var requestDump = loadMockRequest(t)

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	cfg := config.Config{}
	prj := config.Project{
		GitProvider: "gitea",
		Repo:        "religiosa/staticus",
		Actions:     makeActionsList(config.Action{}),
	}

	prj.Actions = append([]config.Action{
		{
			On:     "push",
			Branch: "non-existing",
			Run:    []string{"go", "version"},
		},
	}, prj.Actions...)

	getActionResponse := func(t *testing.T, cfg config.Config) map[string]interface{} {
		t.Helper()
		rcvr := whreceiver.New(prj)
		request := requestDump.ToHttpRequest(projectEndPoint)
		response := httptest.NewRecorder()
		handlers.HandleWebhookPost(ch, logger, cfg, projectName, prj, rcvr)(response, request)

		result := response.Result()

		if result.StatusCode != 201 {
			t.Errorf("Expected status 201 OK, got %v", result.StatusCode)
		}

		body, err := io.ReadAll(result.Body)
		if err != nil {
			t.Fatalf("Failed to read response body: %v", err)
		}

		var responseBody []map[string]interface{}
		if err := json.Unmarshal(body, &responseBody); err != nil {
			t.Fatalf("Failed to unmarshal response body: %v", err)
		}

		if len(responseBody) != 1 {
			t.Errorf("Exepecting one action to match")
		}
		actionResponse := responseBody[0]

		return actionResponse
	}

	t.Run("contains action index and the project name of matched action", func(t *testing.T) {
		actionResponse := getActionResponse(t, cfg)

		var wantIndex float64 = 1
		if actionIndex, ok := actionResponse["actionIdx"].(float64); !ok || actionIndex != wantIndex {
			t.Errorf("Unexpected action index value, got %v, want %v", actionResponse["actionIdx"], wantIndex)
		}

		if prjName, ok := actionResponse["project"].(string); !ok || prjName != projectName {
			t.Errorf("Unexpected project name value, got %v, want %s", actionResponse["project"], projectName)
		}
	})

	t.Run("contains pipeId of run action", func(t *testing.T) {
		actionResponse := getActionResponse(t, cfg)

		if pipeId, ok := actionResponse["pipeId"].(string); !ok || pipeId == "" {
			t.Errorf("No pipeId is present in the response: %v", actionResponse)
		}
	})

	noPublicUrlTests := []struct {
		name   string
		url    string
		config config.Config
	}{
		{"default generation value", "http://localhost:9090/", config.Config{Host: "localhost", Port: 9090}},
		{"host value", "http://example.com:9090/", config.Config{Host: "example.com", Port: 9090}},
		{"port value", "http://localhost:32167/", config.Config{Host: "localhost", Port: 32167}},
		{"partial ssl no cert", "http://localhost:9090/", config.Config{Host: "localhost", Port: 9090, Ssl: config.SslConfig{KeyFilePath: "foo"}}},
		{"partial ssl no key", "http://localhost:9090/", config.Config{Host: "localhost", Port: 9090, Ssl: config.SslConfig{CertFilePath: "bar"}}},
		{"full ssl", "https://localhost:9090/", config.Config{Host: "localhost", Port: 9090, Ssl: config.SslConfig{KeyFilePath: "foo", CertFilePath: "bar"}}},
	}
	for _, tt := range noPublicUrlTests {
		t.Run("contains url field, filled with data from the config protocol, host, and port: "+tt.name, func(t *testing.T) {
			actionResponse := getActionResponse(t, tt.config)
			url, ok := actionResponse["url"].(string)

			if !ok || url == "" {
				t.Errorf("No url field is present in the response: %v", actionResponse)
			}

			want := tt.url + "pipelines/"

			if !strings.HasPrefix(url, want) {
				t.Errorf("Unexpected url value, want prefix: '%s', got '%s'", want, url)
			}
		})
	}

	t.Run("if public url is present, then it overrides values in the url field", func(t *testing.T) {
		publicUrl := "ftp://example.com/"
		actionResponse := getActionResponse(t, config.Config{Host: "localhost", Port: 9090, PublicUrl: publicUrl})
		url, ok := actionResponse["url"].(string)

		if !ok || url == "" {
			t.Errorf("No url field is present in the response: %v", actionResponse)
		}

		want := publicUrl + "pipelines/"

		if !strings.HasPrefix(url, want) {
			t.Errorf("Unexpected url value, want prefix: '%s', got '%s'", want, url)
		}
	})

	t.Run("trailing slash is optional for the public url", func(t *testing.T) {
		publicUrl := "ftp://example.com"
		actionResponse := getActionResponse(t, config.Config{Host: "localhost", Port: 9090, PublicUrl: publicUrl})
		url, ok := actionResponse["url"].(string)

		if !ok || url == "" {
			t.Errorf("No url field is present in the response: %v", actionResponse)
		}

		want := publicUrl + "/" + "pipelines/"

		if !strings.HasPrefix(url, want) {
			t.Errorf("Unexpected url value, want prefix: '%s', got '%s'", want, url)
		}
	})

	t.Run("no url is present if inspection API is disabled", func(t *testing.T) {
		actionResponse := getActionResponse(t, config.Config{
			Host:       "example.com",
			Port:       9090,
			DisableApi: true,
		})

		if url, ok := actionResponse["url"].(string); ok || url != "" {
			t.Errorf("There should be no url field if web admin is disbaled, but got: %s", url)
		}
	})
}
