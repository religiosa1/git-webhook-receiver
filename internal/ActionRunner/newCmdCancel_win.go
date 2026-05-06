//go:build !unix

package ActionRunner

import (
	"os/exec"
	"syscall"
)

func newCanceller(cmd *exec.Cmd, _ *syscall.SysProcAttr) func() error {
	return func() error {
		return cmd.Process.Kill()
	}
}
