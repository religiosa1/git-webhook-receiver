package actionrunner

import "github.com/religiosa1/git-webhook-receiver/internal/config"

type ActionIdentifier struct {
	Index   int    `json:"actionIdx"`
	Project string `json:"project"`
	PipeID  string `json:"pipeId"`
}
type ActionDescriptor struct {
	ActionIdentifier
	Config config.Action `json:"-"`
}

func GetActionIds(descs []ActionDescriptor) []ActionIdentifier {
	result := make([]ActionIdentifier, len(descs))
	for i := range descs {
		result[i] = descs[i].ActionIdentifier
	}
	return result
}
