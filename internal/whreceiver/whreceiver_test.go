package whreceiver_test

import (
	"errors"
	"net/http"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/requestmock"
	"github.com/religiosa1/git-webhook-receiver/internal/whreceiver"
)

func TestReceivers(t *testing.T) {
	receivers := []struct {
		name      string
		project   config.Project
		request   requestmock.RequestMock
		postInfo  whreceiver.WebhookPostInfo
		authToken string
		secret    string
	}{
		{
			name: "gitea",
			project: config.Project{
				GitProvider: "gitea",
				Repo:        "religiosa/staticus",
				Actions: []config.Action{
					{
						On:     "push",
						Branch: "master",
					},
				},
			},
			request: requestmock.LoadRequestMock(t, "../requestmock/captured-requests/gitea.json"),
			postInfo: whreceiver.WebhookPostInfo{
				DeliveryID: "e3b2f0a4-2e9b-417a-b2e5-1a808025998e",
				Branch:     "master",
				Event:      "push",
				Hash:       "323b2c0d7778db8aa4164db5aacea772c4c4feaf",
			},
			authToken: "JgHhtuPOISmw3WDCRtz4H6IrT8zWwNkS",
			secret:    "cc7ec03e-2e09-4bb9-b2fc-388b865200d0",
		},
		{
			name: "github",
			project: config.Project{
				GitProvider: "github",
				Repo:        "religiosa1/github-test",
				Actions: []config.Action{
					{
						On:     "push",
						Branch: "main",
					},
				},
			},
			request: requestmock.LoadRequestMock(t, "../requestmock/captured-requests/github.json"),
			postInfo: whreceiver.WebhookPostInfo{
				DeliveryID: "98105a58-6936-11ef-95df-08616739eae5",
				Branch:     "main",
				Event:      "push",
				Hash:       "92bcfadb4199556415be69b9c31c0dc72343fea2",
			},
			authToken: "",
			secret:    "3216732167",
		},
		{
			name: "gitlab",
			project: config.Project{
				GitProvider: "gitlab",
				Repo:        "root/test",
				Actions: []config.Action{
					{
						On:     "push",
						Branch: "main",
					},
				},
			},
			request: requestmock.LoadRequestMock(t, "../requestmock/captured-requests/gitlab.json"),
			postInfo: whreceiver.WebhookPostInfo{
				DeliveryID: "53c7c667-1a4b-49f2-94e1-8246ab64c776",
				Branch:     "main",
				Event:      "push",
				Hash:       "ecb02fa2ce0400aaa3415de7b5b9f3cb59b3d5dc",
			},
			authToken: "32167",
			secret:    "",
		},
	}

	for _, tt := range receivers {
		rcvr := whreceiver.New(tt.project)
		capabilities := rcvr.GetCapabilities()

		t.Run(tt.name+": rejects incorrect repo in the payload", func(t *testing.T) {
			badProject := tt.project
			badProject.Repo = "bar/ugly"
			rcvr := whreceiver.New(badProject)
			_, err := rcvr.GetWebhookInfo(MakeWebhookPostRequest(tt.request))
			if _, ok := err.(whreceiver.IncorrectRepoError); !ok {
				t.Errorf("expected receiver to reject with IncorrectRepoError, instead got: %s", err)
			}
		})

		t.Run(tt.name+": parses correct WebhookPostInfo", func(t *testing.T) {
			rcvr := whreceiver.New(tt.project)
			got, err := rcvr.GetWebhookInfo(MakeWebhookPostRequest(tt.request))
			if err != nil {
				t.Error(err)
			}

			CompareWebhookPostInfo(t, tt.postInfo, *got)
		})

		if capabilities.CanAuthorize {
			t.Run(tt.name+": good auth token", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				got, err := rcvr.Authorize(MakeWebhookPostRequest(tt.request), tt.authToken)
				if err != nil {
					t.Error(err)
				}
				if got != true {
					t.Errorf("Authorization test failed, got %t, want true", got)
				}
			})
			t.Run(tt.name+": bad auth token", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				got, err := rcvr.Authorize(MakeWebhookPostRequest(tt.request), "bad token")
				if err != nil {
					t.Error(err)
				}
				if got != false {
					t.Errorf("Authorization test failed, got %t, want false", got)
				}
			})
		} else {
			t.Run(tt.name+": authorizatoin not supported", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				_, err := rcvr.Authorize(MakeWebhookPostRequest(tt.request), tt.authToken)
				if err == nil {
					t.Error("Exepcted authorization to throw but it didn't")
				}
				if !errors.Is(err, whreceiver.ErrAuthNotSupported) {
					t.Errorf("Unexepcted error type, must be ErrAuthNotSupported, got: %s", err)
				}
			})
		}

		if capabilities.CanVerifySignature {
			t.Run(tt.name+": good signature", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				req := MakeWebhookPostRequest(tt.request)
				got, err := rcvr.VerifySignature(req, tt.secret)
				if err != nil {
					t.Error(err)
				}
				if got != true {
					t.Errorf("Secret validation failed, got %t, want true", got)
				}
			})
			t.Run(tt.name+": bad signature", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				req := MakeWebhookPostRequest(tt.request)
				got, err := rcvr.VerifySignature(req, "bad secret")
				if err != nil {
					t.Error(err)
				}
				if got != false {
					t.Errorf("Secret validation failed, got %t, want false", got)
				}
			})
		} else {
			t.Run(tt.name+": signature not supported", func(t *testing.T) {
				rcvr := whreceiver.New(tt.project)
				req := MakeWebhookPostRequest(tt.request)
				_, err := rcvr.VerifySignature(req, tt.secret)
				if err == nil {
					t.Errorf("Expected signature verification to throw, but it didn't")
				}
				if !errors.Is(err, whreceiver.ErrSignNotSupported) {
					t.Errorf("Unexepcted error type, must be ErrSignNotSupported, got: %s", err)
				}
			})
		}
		// TODO ping tests
	}
}

func MakeWebhookPostRequest(requestMock requestmock.RequestMock) (req whreceiver.WebhookPostRequest) {
	req.Payload = []byte(requestMock.Body)
	req.Headers = http.Header{}
	for key, value := range requestMock.Headers {
		req.Headers.Set(key, value)
	}
	return req
}

func CompareWebhookPostInfo(t *testing.T, want whreceiver.WebhookPostInfo, got whreceiver.WebhookPostInfo) {
	t.Helper()
	if want, got := want.DeliveryID, got.DeliveryID; want != got {
		t.Errorf("Unexpected DeliveryID value, want '%s', got '%s'", want, got)
	}
	if want, got := want.Branch, got.Branch; want != got {
		t.Errorf("Unexpected Branch value, want '%s', got '%s'", want, got)
	}
	if want, got := want.Event, got.Event; want != got {
		t.Errorf("Unexpected Event value, want '%s', got '%s'", want, got)
	}
	if want, got := want.Hash, got.Hash; want != got {
		t.Errorf("Unexpected Hash value, want '%s', got '%s'", want, got)
	}
}
