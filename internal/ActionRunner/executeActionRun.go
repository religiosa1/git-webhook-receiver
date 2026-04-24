package ActionRunner

import (
	"context"
	"io"
	"syscall"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

func executeActionRun(ctx context.Context, action config.Action, sysProcAttr *syscall.SysProcAttr, output io.Writer) error {
	gracefulKillTimeout := time.Duration(action.GracefulShutdownMS) * time.Millisecond
	cmd := newCmd(ctx, action.Run[0], action.Run[1:], sysProcAttr, gracefulKillTimeout)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}
	cmd.Stdout = output
	cmd.Stderr = output
	return cmd.Run()
}
