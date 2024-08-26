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

type Receiver interface {
	Authorize(req WebhookPostRequest, auth string) (bool, error)
	VerifySignature(req WebhookPostRequest, secret string) (bool, error)
	IsPingRequest(req WebhookPostRequest) bool
	GetWebhookInfo(WebhookPostRequest) (postInfo *WebhookPostInfo, err error)
}

func New(project config.Project) Receiver {
	var receiver Receiver
	switch project.GitProvider {
	case "gitea":
		receiver = GiteaReceiver{project}
	case "github":
		receiver = GithubReceiver{project}
	}
	return receiver
}
