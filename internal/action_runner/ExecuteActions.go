package action_runner

import (
	"log/slog"
)

func ExecuteActions(
	logger *slog.Logger,
	actionDescriptors []ActionDescriptor,
	actions_output_dir string,
) {
	for _, actionDescriptor := range actionDescriptors {
		pipeLogger := logger.With(slog.String("pipeId", actionDescriptor.PipeId))
		pipeLogger.Info("Running action", slog.Int("action_index", actionDescriptor.Index))
		streams, err := getActionIoStreams(actions_output_dir, actionDescriptor.PipeId, pipeLogger)
		if err != nil {
			pipeLogger.Error("Error creating action's IO streams", slog.Any("error", err))
			continue
		}
		defer streams.Close()
		if len(actionDescriptor.Action.Run) > 0 {
			executeActionRun(pipeLogger.With(slog.Any("command", actionDescriptor.Action.Run)), actionDescriptor.Action, streams)
		} else {
			executeActionScript(pipeLogger, actionDescriptor.Action, streams)
		}
	}
}
