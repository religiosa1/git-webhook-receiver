package ActionRunner

import (
	"context"
	"os"
	"os/exec"
	"runtime"
	"syscall"
	"time"
)

const processKillTimeout = 30 * time.Second

// newCmd creates an exec.Cmd with graceful cancellation: SIGINT on Unix (SIGKILL
// on Windows), followed by a hard kill after processKillTimeout if the process
// hasn't exited. path is the resolved executable; args are the arguments without
// the command name (i.e. args[1:] in shell handler terms).
func newCmd(ctx context.Context, path string, args []string, sysProcAttr *syscall.SysProcAttr) *exec.Cmd {
	cmd := exec.CommandContext(ctx, path, args...)
	cmd.SysProcAttr = sysProcAttr
	cmd.Cancel = func() error {
		if runtime.GOOS == "windows" {
			return cmd.Process.Kill()
		}
		return cmd.Process.Signal(os.Interrupt)
	}
	cmd.WaitDelay = processKillTimeout
	return cmd
}
