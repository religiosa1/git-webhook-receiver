package whreceiver

import (
	"net/http"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

type WebhookPostInfo struct {
	DeliveryID string
	Branch     string
	Event      string
	Hash       string
	Auth       string
}

type Receiver interface {
	GetWebhookInfo(*http.Request) (postInfo *WebhookPostInfo, err error)
}

func New(project *config.Project) Receiver {
	var receiver Receiver
	switch project.GitProvider {
	case "gitea":
		receiver = GiteaReceiver{project}
	}
	return receiver
}
