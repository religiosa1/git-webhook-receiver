package logger

import (
	"log"
	"log/slog"
	"os"

	slogmulti "github.com/samber/slog-multi"
)

type ClosableLogger struct {
	Logger *slog.Logger
	file   *os.File
}

func (logger ClosableLogger) Close() {
	if logger.file != nil {
		logger.file.Close()
	}
}

func SetupLogger(logLevel string, logFileName string) (ClosableLogger, error) {
	var programLevel = new(slog.LevelVar)
	programLevel.Set(strLogLevelToEnumValue(logLevel))
	hdlrOpts := &slog.HandlerOptions{Level: programLevel}

	textHandler := slog.NewTextHandler(os.Stdout, hdlrOpts)
	if logFileName == "" {
		return ClosableLogger{slog.New(textHandler), nil}, nil
	}

	file, err := os.OpenFile(logFileName, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return ClosableLogger{}, err
	}

	return ClosableLogger{
		slog.New(slogmulti.Fanout(
			textHandler,
			slog.NewJSONHandler(file, hdlrOpts),
		)), file}, nil
}

func strLogLevelToEnumValue(logLevel string) slog.Level {
	switch logLevel {
	case "debug":
		return slog.LevelDebug
	case "info":
		return slog.LevelInfo
	case "warn":
		return slog.LevelWarn
	case "error":
		return slog.LevelError
	default:
		log.Fatalf("Unexpected log level %s", logLevel)
		return slog.LevelInfo
	}
}
