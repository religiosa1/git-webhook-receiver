package wh_receiver

import (
	"net/http"

	"github.com/religiosa1/deployer/internal/config"
)

type Receiver interface {
	ExtractAction(*http.Request) (action *config.Action, err error)
}

func New(project *config.Project) Receiver {
	var receiver Receiver
	switch project.GitProvider {
	case "gitea":
		receiver = GiteaReceiver{project}
	}
	return receiver
}
