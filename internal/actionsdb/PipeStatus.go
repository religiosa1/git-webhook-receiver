package actionsdb

import "fmt"

type PipeStatus int

const (
	PipeStatusAny PipeStatus = iota
	PipeStatusOk
	PipeStatusError
	PipeStatusPending
)

func ParsePipelineStatus(status string) (PipeStatus, error) {
	switch status {
	case "ok":
		return PipeStatusOk, nil
	case "error":
		return PipeStatusError, nil
	case "pending":
		return PipeStatusPending, nil
	case "", "any":
		return PipeStatusAny, nil
	default:
		return PipeStatusAny, fmt.Errorf("unknown pipe status: %q", status)
	}
}

func (s PipeStatus) String() string {
	switch s {
	case PipeStatusOk:
		return "ok"
	case PipeStatusError:
		return "error"
	case PipeStatusPending:
		return "pending"
	default:
		return ""
	}
}
