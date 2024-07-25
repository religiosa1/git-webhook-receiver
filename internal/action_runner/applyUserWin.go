//go:build !unix

package action_runner

import (
	"errors"
	"os/exec"
)

func applyUser(string, *exec.Cmd) error {
	return errors.New("setting exec user is not supported on non-unix env")
}
