//go:build unix

package ActionRunner

import (
	"os"
	"os/exec"
	"syscall"
)

func newCanceller(cmd *exec.Cmd, sysProcAttr *syscall.SysProcAttr) func() error {
	return func() error {
		// When the process was started in its own process group (Setpgid), kill
		// the whole group so that child processes also receive SIGINT and exit
		// immediately instead of outliving their parent.
		if sysProcAttr != nil && sysProcAttr.Setpgid {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		}
		return cmd.Process.Signal(os.Interrupt)
	}
}
