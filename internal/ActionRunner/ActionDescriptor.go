package ActionRunner

import "github.com/religiosa1/git-webhook-receiver/internal/config"

type ActionIdentifier struct {
	Index      int    `json:"actionIdx"`
	Project    string `json:"project"`
	PipeId     string `json:"pipeId"`
	DeliveryId string `json:"-"`
}
type ActionDescriptor struct {
	ActionIdentifier
	Action config.Action `json:"-"`
}

func GetActionIds(descs []ActionDescriptor) []ActionIdentifier {
	result := make([]ActionIdentifier, len(descs))
	for i := 0; i < len(descs); i++ {
		result[i] = descs[i].ActionIdentifier
	}
	return result
}
