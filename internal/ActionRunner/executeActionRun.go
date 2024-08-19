package ActionRunner

import (
	"context"
	"io"
	"os"
	"os/exec"
	"syscall"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func executeActionRun(ctx context.Context, action config.Action, sysProcAttr *syscall.SysProcAttr, output io.Writer) error {
	cmd := exec.Command(action.Run[0], action.Run[1:]...)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}

	cmd.SysProcAttr = sysProcAttr

	cmd.Stdout = output
	cmd.Stderr = output

	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			_ = cmd.Process.Signal(os.Interrupt)
		}()
	}

	return cmd.Run()
}
