package ActionRunner

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"time"
)

func (runner ActionRunner) executeAction(
	logger *slog.Logger,
	actionDescriptor ActionDescriptor,
) {
	action := actionDescriptor.Action
	pipeLogger := logger.With(slog.String("pipeId", actionDescriptor.PipeID))
	pipeLogger.Info("Running action", slog.Int("action_index", actionDescriptor.Index))

	if runner.actionsDB != nil {
		err := runner.actionsDB.CreateRecord(actionDescriptor.PipeID, actionDescriptor.Project, actionDescriptor.DeliveryID, action)
		if err != nil {
			pipeLogger.Error("Error creating pipeline record in the db", slog.Any("error", err))
			return
		}
	}

	output, err := os.CreateTemp("", actionDescriptor.PipeID+"-*.output.tmp")
	if err != nil {
		pipeLogger.Error("Error creating temporary file to capture action's output", slog.Any("error", err))
		return
	}
	defer func() {
		err := output.Close()
		if err != nil {
			pipeLogger.Error("Error closing action output", slog.Any("error", err))
		}
	}()

	sysProcAttr, err := getSysProcAttr(action.User)
	if err != nil {
		logger.Error("Error creating process attributes for action", slog.Any("error", err))
		return
	}

	if action.User != "" {
		logger.Debug("Running from a user", slog.String("user", action.User))
	}

	actionCtx, cancelAction := context.WithTimeout(runner.ctx, time.Duration(action.TimeoutSeconds)*time.Second)
	defer cancelAction()

	var actionErr error
	if len(action.Run) > 0 {
		logger.Debug("Running the command", slog.Any("command", action.Run))
		actionErr = executeActionRun(actionCtx, action, sysProcAttr, output)
	} else {
		logger.Debug("Running the script", slog.String("script", action.Script))
		actionErr = executeActionScript(actionCtx, action, sysProcAttr, output)
	}

	if actionErr != nil {
		logger.Error("Error while running the action", slog.Any("error", actionErr))
	} else {
		logger.Info("Action successfully finished")
	}

	content, err := readOutputFile(output)
	if err != nil {
		logger.Error("Error while reading output file's content", slog.Any("error", err))
	}

	if runner.actionsDB != nil {
		err := runner.actionsDB.CloseRecord(actionDescriptor.PipeID, actionErr, content)
		if err != nil {
			pipeLogger.Error("Error closing action's db record", slog.Any("error", err))
			return
		}
	}
}

func readOutputFile(output *os.File) (string, error) {
	_, err := output.Seek(0, io.SeekStart)
	if err != nil {
		return "", fmt.Errorf("seeking error: %w", err)
	}
	content, err := io.ReadAll(output)
	return string(content), err
}
