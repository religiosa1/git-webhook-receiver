package action_runner

import (
	"log/slog"
	"os/exec"
	"runtime"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func executeActionRun(logger *slog.Logger, action config.Action, streams actionIoStreams) {
	logger.Debug("Running the command", slog.Any("command", action.Run))
	cmd := exec.Command(action.Run[0], action.Run[1:]...)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}

	if runtime.GOOS != "windows" && action.User != "" {
		if err := applyUser(action.User, cmd); err != nil {
			logger.Error(
				"Unable to run action from the specified user:",
				slog.String("username", action.User),
				slog.Any("error", err),
			)
		} else {
			logger.Debug("Running the command from a user", slog.String("user", action.User))
		}
	}

	cmd.Stdout = streams.Stdout
	cmd.Stderr = streams.Stderr

	if err := cmd.Run(); err != nil {
		logger.Error("Command execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Command successfully finished")
	}
}
