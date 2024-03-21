package wh_receiver

import (
	"net/http"

	"github.com/religiosa1/deployer/internal/config"
)

type WebhookPostInfo struct {
	Branch string
	Event  string
	Hash   string
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
