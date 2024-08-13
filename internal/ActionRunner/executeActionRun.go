package ActionRunner

import (
	"context"
	"log/slog"
	"os"
	"os/exec"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func executeActionRun(ctx context.Context, logger *slog.Logger, action config.Action, streams actionIoStreams) {
	logger.Debug("Running the command", slog.Any("command", action.Run))
	cmd := exec.Command(action.Run[0], action.Run[1:]...)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}

	sysProcAttr, err := getSysProcAttr(action.User)
	if err != nil {
		logger.Error("Error creating process attributes for action", slog.Any("error", err))

	}
	if action.User != "" {
		logger.Debug("Running the command from a user", slog.String("user", action.User))
	}
	cmd.SysProcAttr = sysProcAttr

	cmd.Stdout = streams.Stdout
	cmd.Stderr = streams.Stderr

	if done := ctx.Done(); done != nil {
		go func() {
			<-done
			_ = cmd.Process.Signal(os.Interrupt)
		}()
	}

	if err := cmd.Run(); err != nil {
		logger.Error("Command execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Command successfully finished")
	}
}
