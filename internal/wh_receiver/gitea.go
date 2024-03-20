package wh_receiver

import (
	"errors"
	"net/http"

	"github.com/religiosa1/deployer/internal/config"
)

type GiteaReceiver struct {
	project *config.Project
}

func (receiver GiteaReceiver) ExtractAction(req *http.Request) (action *config.Action, err error) {
	return nil, errors.New("Unimplemented")
}
