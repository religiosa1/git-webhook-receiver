package handlers

import (
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/religiosa1/webhook-receiver/internal/config"
)

func executeActionRun(logger *slog.Logger, action config.Action, pipeId string, actions_output_dir string) {
	logger.Info("Running the command")
	cmd := exec.Command(action.Run[0], action.Run[1:]...)
	if action.Cwd != "" {
		cmd.Dir = action.Cwd
	}

	if actions_output_dir != "" {
		closer, err := redirectCmdOutputStreams(cmd, actions_output_dir, pipeId, logger)
		if err != nil {
			logger.Error("Error redirecting command IO streams", slog.Any("error", err))
			return
		}
		defer closer.Close()
	}

	err := cmd.Run()
	if err != nil {
		logger.Error("Command execution ended with an error", slog.Any("error", err))
	} else {
		logger.Info("Command successfully finished")
	}
}

type redirectedStreamCloser struct {
	logger         *slog.Logger
	stdoutFile     *os.File
	stdoutFileName string
	stderrFile     *os.File
	stderrFileName string
}

func (closer redirectedStreamCloser) Close() {
	if closer.stdoutFile != nil {
		closer.stdoutFile.Close()
		removeFileIfEmpty(closer.stdoutFileName, closer.logger)
	}
	if closer.stderrFile != nil {
		closer.stderrFile.Close()
		removeFileIfEmpty(closer.stderrFileName, closer.logger)
	}
}

func redirectCmdOutputStreams(
	cmd *exec.Cmd,
	actions_output_dir string,
	pipeId string,
	logger *slog.Logger,
) (redirectedStreamCloser, error) {
	stdoutFileName := filepath.Join(actions_output_dir, pipeId+".stdout")
	stdoutFile, err := os.OpenFile(stdoutFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		logger.Error("Unable to open stdout file for action", slog.Any("error", err))
		return redirectedStreamCloser{}, err
	}

	stderrFileName := filepath.Join(actions_output_dir, pipeId+".stderr")
	stderrFile, err := os.OpenFile(stderrFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		stdoutFile.Close()
		logger.Error("Unable to open stdout file for action", slog.Any("error", err))
		return redirectedStreamCloser{logger, stdoutFile, stdoutFileName, nil, ""}, err
	}
	cmd.Stdout = stdoutFile
	cmd.Stderr = stderrFile
	logger.Debug(
		"Command IO streams redirected",
		slog.String("stdout", stdoutFileName),
		slog.String("stderr", stderrFileName),
	)

	return redirectedStreamCloser{logger, stdoutFile, stdoutFileName, stderrFile, stderrFileName}, nil
}

func removeFileIfEmpty(fileName string, logger *slog.Logger) {
	flogger := logger.With(slog.String("filename", fileName))
	stdoutInfo, err := os.Stat(fileName)
	if err != nil {
		flogger.Error("Unable to get output size stats", slog.Any("error", err))
	} else if stdoutInfo.Size() == 0 {
		err = os.Remove(fileName)
		if err != nil {
			flogger.Error("Failed to delete file", slog.Any("error", err))
		}
	}
}
