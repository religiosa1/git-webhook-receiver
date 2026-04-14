package whreceiver

import (
	"net/http"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type WebhookPostRequest struct {
	Payload []byte
	Headers http.Header
}

type WebhookPostInfo struct {
	DeliveryID string
	Branch     string
	Event      string
	// commit hash-id after applying the event ("after" field of payload)
	Hash string
}

type ReceiverCapabilities struct {
	// Receiver can authorize requests through the Authorization header
	CanAuthorize bool
	// Receiver can verify payload signature in header
	CanVerifySignature bool
	// Receiver supports ping requests to webhook
	HasPing bool
}

type Receiver interface {
	Authorize(req WebhookPostRequest, auth string) (bool, error)
	VerifySignature(req WebhookPostRequest, secret string) (bool, error)
	IsPingRequest(req WebhookPostRequest) bool
	GetWebhookInfo(req WebhookPostRequest) (*WebhookPostInfo, error)
	GetCapabilities() ReceiverCapabilities
}

func New(project config.Project) Receiver {
	var receiver Receiver
	switch project.GitProvider {
	case "gitea":
		receiver = GiteaReceiver{project}
	case "github":
		receiver = GithubReceiver{project}
	case "gitlab":
		receiver = GitlabReceiver{project}
	}

	return receiver
}
