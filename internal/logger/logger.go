package logger

import (
	"log"
	"log/slog"
	"os"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	slogmulti "github.com/samber/slog-multi"
)

func SetupLogger(logLevel string, db *logsDb.LogsDB) (*slog.Logger, error) {
	programLevel := new(slog.LevelVar)
	programLevel.Set(strLogLevelToEnumValue(logLevel))
	handlerOpts := &slog.HandlerOptions{Level: programLevel}

	jsonHandler := slog.NewJSONHandler(os.Stdout, handlerOpts)

	if db == nil {
		return slog.New(jsonHandler), nil
	}

	logger := slog.New(slogmulti.Fanout(
		jsonHandler,
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
