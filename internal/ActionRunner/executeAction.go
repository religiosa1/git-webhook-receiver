package ActionRunner

import (
	"context"
	"io"
	"log/slog"
	"os"
	"strings"
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
		err = os.Remove(output.Name())
		if err != nil {
			pipeLogger.Error("Error removing action output tmp file", slog.Any("error", err))
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

	if runner.actionsDB != nil {
		var outputForDb io.Reader
		if _, err := output.Seek(0, io.SeekStart); err != nil {
			logger.Error("Error seeking output file", slog.Any("error", err))
			outputForDb = strings.NewReader("CORRUPTED")
		} else {
			outputForDb = output
		}
		err := runner.actionsDB.CloseRecord(actionDescriptor.PipeID, actionErr, outputForDb)
		if err != nil {
			pipeLogger.Error("Error closing action's db record", slog.Any("error", err))
			return
		}
	}
}
