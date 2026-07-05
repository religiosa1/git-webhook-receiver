package actionrunner

import "github.com/religiosa1/git-webhook-receiver/internal/config"

// ActionIdentifier is the minimal information required to fully qualify a pipeline:
// project, actionIdx and pipeline run ID
type ActionIdentifier struct {
	Index   int    `json:"actionIdx"`
	Project string `json:"project"`
	PipeID  string `json:"pipeId"`
}

// ActionDescriptor is the information from config, fully describing an action
type ActionDescriptor struct {
	ActionIdentifier
	GitProvider string        `json:"gitProvider"`
	Repo        string        `json:"repo"`
	Config      config.Action `json:"-"`
}
