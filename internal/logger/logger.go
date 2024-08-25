package logger

import (
	"log"
	"log/slog"
	"os"

	"github.com/religiosa1/webhook-receiver/internal/logsDb"
	slogmulti "github.com/samber/slog-multi"
)

func SetupLogger(logLevel string, db *logsDb.LogsDb) (*slog.Logger, error) {
	var programLevel = new(slog.LevelVar)
	programLevel.Set(strLogLevelToEnumValue(logLevel))
	hdlrOpts := &slog.HandlerOptions{Level: programLevel}

	textHandler := slog.NewTextHandler(os.Stdout, hdlrOpts)

	if db == nil {
		return slog.New(textHandler), nil
	}

	logger := slog.New(slogmulti.Fanout(
		textHandler,
		NewDBLogger(db, hdlrOpts),
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
