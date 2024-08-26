package whreceiver_test

import (
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

func TestGithub(t *testing.T) {
	makeRequest := func() (req whreceiver.WebhookPostRequest) {
		req.Payload = []byte(
			`{"ref":"master","after":"aa1a2860561471a17b3b49b4216390d61b196c78",` +
				`"repository":{"full_name":"test/repo"}}`,
		)
		req.Headers = http.Header{}
		req.Headers.Set("Content-Type", "application/json")
		req.Headers.Set("X-GitHub-Event", "push")
		req.Headers.Set("X-GitHub-Delivery", "test-delivery-id")
		return
	}

	var githubProject = config.Project{
		GitProvider: "github",
		Repo:        "test/repo",
		Actions: []config.Action{
			{
				On:     "push",
				Branch: "master",
			},
		},
	}

	t.Run("GetWebhookInfo", func(t *testing.T) {
		rcvr := whreceiver.New(githubProject)
		got, err := rcvr.GetWebhookInfo(makeRequest())
		if err != nil {
			t.Errorf("Error during auth test %s", err)
		}
		want := &whreceiver.WebhookPostInfo{
			DeliveryID: "test-delivery-id",
			Branch:     "master",
			Event:      "push",
			Hash:       "aa1a2860561471a17b3b49b4216390d61b196c78",
		}

		if *got != *want {
			t.Errorf("GetWebhookInfo test failed, got %v, want %v", got, want)
		}
	})

	t.Run("Authorization returns an error", func(t *testing.T) {
		rcvr := whreceiver.New(githubProject)
		_, err := rcvr.Authorize(makeRequest(), "asdfasdf")
		if err == nil {
			t.Error("Exepcted authorization to throw but it didn't")
		}
	})

	t.Run("VerifySignature", func(t *testing.T) {
		secret := "123456"
		signature := hex.EncodeToString(whreceiver.GetPayloadSignature(secret, makeRequest().Payload))
		secretTests := []struct {
			name   string
			secret string
			want   bool
		}{
			{"correct signature passed", secret, true},
			{"mismatched signature", "bad secret", false},
		}
		for _, tt := range secretTests {
			t.Run(tt.name, func(t *testing.T) {
				t.Parallel()
				rcvr := whreceiver.New(githubProject)
				req := makeRequest()
				req.Headers.Add("X-Hub-Signature-256", "sha256="+signature)

				got, err := rcvr.VerifySignature(req, tt.secret)
				if err != nil {
					t.Errorf("Error during secret test: %s", err)
				}
				if got != tt.want {
					t.Errorf("Secret validatoin failed, got %t, want %t", got, tt.want)
				}
			})
		}
	})

	pingRequestsTest := []struct {
		name      string
		eventType string
		want      bool
	}{
		{"Returns true on ping requests", "ping", true},
		{"Returns false on non-ping requests", "push", false},
	}
	for _, tt := range pingRequestsTest {
		t.Run(tt.name, func(t *testing.T) {
			rcvr := whreceiver.New(githubProject)
			rqst := makeRequest()
			rqst.Headers.Set("X-GitHub-Event", tt.eventType)
			got := rcvr.IsPingRequest(rqst)
			if got != tt.want {
				t.Errorf("Unexpected result, got %t want %t", got, tt.want)
			}
		})
	}
}
