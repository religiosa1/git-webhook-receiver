package ActionRunner

import (
	"log/slog"
	"os"
	"path/filepath"
)

type actionIoStreams struct {
	logger         *slog.Logger
	Stdout         *os.File
	stdoutFileName string
	Stderr         *os.File
	stderrFileName string
}

func (closer actionIoStreams) Close() {
	if closer.Stdout != nil {
		closer.Stdout.Close()
		removeFileIfEmpty(closer.stdoutFileName, closer.logger)
	}
	if closer.Stderr != nil {
		closer.Stderr.Close()
		removeFileIfEmpty(closer.stderrFileName, closer.logger)
	}
}

func getActionIoStreams(actions_output_dir string, pipeId string, logger *slog.Logger) (actionIoStreams, error) {
	if actions_output_dir == "" {
		return actionIoStreams{}, nil
	}
	stdoutFileName := filepath.Join(actions_output_dir, pipeId+".stdout")
	stdoutFile, err := os.OpenFile(stdoutFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return actionIoStreams{}, err
	}

	stderrFileName := filepath.Join(actions_output_dir, pipeId+".stderr")
	stderrFile, err := os.OpenFile(stderrFileName, os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		stdoutFile.Close()
		return actionIoStreams{logger, stdoutFile, stdoutFileName, nil, ""}, err
	}

	return actionIoStreams{logger, stdoutFile, stdoutFileName, stderrFile, stderrFileName}, nil
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
