package ActionRunner

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

// newCmd creates an exec.Cmd with graceful cancellation: SIGINT on Unix (SIGKILL
// on Windows), followed by a hard kill after gracefulKillTimeout if the process
// hasn't exited. path is the resolved executable; args are the arguments without
// the command name (i.e. args[1:] in shell handler terms).
func newCmd(ctx context.Context, path string, args []string, sysProcAttr *syscall.SysProcAttr, gracefulKillTimeout time.Duration) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = sysProcAttr
	cmd.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		// When the process was started in its own process group (Setpgid), kill
		// the whole group so that child processes also receive SIGINT and exit
		// immediately instead of outliving their parent.
		if sysProcAttr != nil && sysProcAttr.Setpgid {
			return syscall.Kill(-cmd.Process.Pid, syscall.SIGINT)
		}
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = gracefulKillTimeout
	return cmd
}
