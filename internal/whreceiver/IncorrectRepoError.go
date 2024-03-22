package whreceiver

import "fmt"

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
