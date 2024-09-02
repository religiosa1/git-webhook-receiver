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
		"Incorrect repo receieved in the webhook payload, expected '%s' but received '%s'",
		err.Expected,
		err.Actual,
	)
}

type AuthorizationError struct {
	info string
}

func (err AuthorizationError) Error() string {
	return fmt.Sprintf(
		"Incorrect authorization information passed to action: '%s'",
		err.info,
	)
}

var ErrAuthNotSupported = errors.New("authorization header is not supported for this receiver, use secret signature instead")
var ErrSignNotSupported = errors.New("request signature is not supported for this receiver, use authorization header instead")
