package whreceiver_test

import (
	"encoding/hex"
	"net/http"
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"github.com/religiosa1/webhook-receiver/internal/whreceiver"
)

var testRequest = whreceiver.WebhookPostRequest{
	Payload: []byte(
		`{"ref":"master","after":"aa1a2860561471a17b3b49b4216390d61b196c78",` +
			`"repository":{"full_name":"test/repo"}}`,
	),
	Headers: http.Header{
		"Content-Type":     {"application/json"},
		"X-Gitea-Event":    {"push"},
		"X-Gitea-Delivery": {"test-delivery-id"},
		"Authorization":    {"pass1"},
	},
}

var Project = config.Project{
	GitProvider: "gitea",
	Repo:        "test/repo",
	Actions: []config.Action{
		{
			On:            "push",
			Branch:        "master",
			Authorization: "pass1",
		},
	},
}

func TestGetWebhookInfo(t *testing.T) {
	rcvr := whreceiver.New(&Project)
	got, err := rcvr.GetWebhookInfo(testRequest)
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
}

func TestAuthorization(t *testing.T) {
	authTests := []struct {
		name  string
		token string
		want  bool
	}{
		{"good password token", "pass1", true},
		{"bad password token", "bad pass", false},
	}
	for _, tt := range authTests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			rcvr := whreceiver.New(&Project)
			got, err := rcvr.Authorize(testRequest, tt.token)
			if err != nil {
				t.Errorf("Error during auth test %s", err)
			}
			if got != tt.want {
				t.Errorf("Authorization test failed, got %t, want %t", got, tt.want)
			}
		})
	}
}

func TestValidateSignature(t *testing.T) {
	secret := "123456"
	signature := hex.EncodeToString(whreceiver.GetPayloadSignature(secret, testRequest.Payload))
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
			rcvr := whreceiver.New(&Project)
			req := testRequest
			req.Headers = testRequest.Headers.Clone()
			req.Headers.Add("X-Gitea-Signature", signature)

			got, err := rcvr.ValidateSignature(req, tt.secret)
			if err != nil {
				t.Errorf("Error during secret test %s", err)
			}
			if got != tt.want {
				t.Errorf("Secret validatoin failed, got %t, want %t", got, tt.want)
			}
		})
	}
}
