package whreceiver

import (
	"errors"
	"fmt"
)

type IncorrectRepoError struct {
	Expected string
	Actual   string
}

func (err IncorrectRepoError) Error() string {
	return fmt.Sprintf(
		"Incorrect repo received in the webhook payload, expected %q but received %q",
		err.Expected,
		err.Actual,
	)
}

var (
	ErrAuthNotSupported = errors.New("authorization header is not supported for this receiver, use secret signature instead")
	ErrSignNotSupported = errors.New("request signature is not supported for this receiver, use authorization header instead")
)
