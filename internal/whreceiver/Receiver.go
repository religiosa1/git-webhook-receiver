package whreceiver

import (
	"fmt"
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
	default:
		panic(fmt.Sprintf("unknown receiver provided: %q", project.GitProvider))
	}

	return receiver
}

// Capabilities returns the receiver capabilities for a project's provider.
// Unlike New it does not panic on an unknown provider, returning zero
// capabilities instead, so it's safe to call from views.
func Capabilities(project config.Project) ReceiverCapabilities {
	switch project.GitProvider {
	case "gitea":
		return GiteaReceiver{project}.GetCapabilities()
	case "github":
		return GithubReceiver{project}.GetCapabilities()
	case "gitlab":
		return GitlabReceiver{project}.GetCapabilities()
	default:
		return ReceiverCapabilities{}
	}
}
