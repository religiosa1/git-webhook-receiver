package actiondb

import "fmt"

type PipeStatus int

const (
	PipeStatusAny     PipeStatus = 0
	PipeStatusOk      PipeStatus = 1
	PipeStatusError   PipeStatus = 2
	PipeStatusPending PipeStatus = 3
)

func ParsePipelineStatus(status string) (PipeStatus, error) {
	switch status {
	case "ok":
		return PipeStatusOk, nil
	case "error":
		return PipeStatusError, nil
	case "pending":
		return PipeStatusPending, nil
	case "any":
		return PipeStatusAny, nil
	default:
		return PipeStatusAny, fmt.Errorf("unknown pipe status: '%s'", status)
	}
}
