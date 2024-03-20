package logger

import (
	"log"
	"log/slog"
	"os"

	slogmulti "github.com/samber/slog-multi"
)

func SetupLogger(logLevel string, logFile *os.File) *slog.Logger {
	var programLevel = new(slog.LevelVar)
	programLevel.Set(strLogLevelToEnumValue(logLevel))
	hdlrOpts := &slog.HandlerOptions{Level: programLevel}

	return slog.New(
		slogmulti.Fanout(
			slog.NewTextHandler(os.Stdout, hdlrOpts),
			slog.NewJSONHandler(logFile, hdlrOpts),
		),
	)
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
