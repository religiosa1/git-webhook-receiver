package action_runner

import (
	"context"
	"log/slog"
	"strings"

	"github.com/religiosa1/webhook-receiver/internal/config"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

func executeActionScript(logger *slog.Logger, action config.Action, streams actionIoStreams) {
	logger.Debug("Running script", slog.String("script", action.Script))
	script, err := syntax.NewParser().Parse(strings.NewReader(action.Script), "")
	if err != nil {
		logger.Error("Error parsing action's script", slog.Any("error", err))
		return
	}

	runner, _ := interp.New(
		interp.StdIO(nil, streams.Stdout, streams.Stderr),
		interp.Dir(action.Cwd),
	)
	if err := runner.Run(context.TODO(), script); err != nil {
		logger.Error("Script execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Script successfully finished")
	}
}
