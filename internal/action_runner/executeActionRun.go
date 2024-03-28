package action_runner

import (
	"log/slog"
	"os/exec"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func executeActionRun(logger *slog.Logger, action config.Action, streams actionIoStreams) {
	logger.Debug("Running the command", slog.Any("command", action.Run))
	cmd := exec.Command(action.Run[0], action.Run[1:]...)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}

	cmd.Stdout = streams.Stdout
	cmd.Stderr = streams.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Command execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Command successfully finished")
	}
}
