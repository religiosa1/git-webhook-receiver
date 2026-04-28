package logger

import (
	"fmt"
	"log"
	"log/slog"
	"os"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	slogmulti "github.com/samber/slog-multi"
)

func SetupLogger(logLevel, logType string, db *logsDb.LogsDB) (*slog.Logger, error) {
	programLevel := new(slog.LevelVar)
	programLevel.Set(strLogLevelToEnumValue(logLevel))
	handlerOpts := &slog.HandlerOptions{Level: programLevel}

	var handler slog.Handler
	switch logType {
	case "json":
		handler = slog.NewJSONHandler(os.Stdout, handlerOpts)
	case "text":
		handler = slog.NewTextHandler(os.Stdout, handlerOpts)
	default:
		return nil, fmt.Errorf("unknown logger type: %q", logType)
	}

	if db == nil {
		return slog.New(handler), nil
	}

	logger := slog.New(slogmulti.Fanout(
		handler,
		NewDBLogger(db, handlerOpts),
	))

	return logger, nil
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
