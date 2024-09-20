package serialization

import (
	"log/slog"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
)

type PrettyLogEntry struct {
	Level      string     `json:"level"`
	Project    NullString `json:"project"`
	DeliveryId NullString `json:"deliveryId"`
	PipeId     NullString `json:"pipeId"`
	Message    string     `json:"message"`
	Data       JsonData   `json:"data"`
	Ts         Timestamp  `json:"ts"`
}

func LogEntry(e logsDb.LogEntry) PrettyLogEntry {
	var level string
	switch slog.Level(e.Level) {
	case slog.LevelDebug:
		level = "debug"
	case slog.LevelInfo:
		level = "info"
	case slog.LevelWarn:
		level = "warn"
	case slog.LevelError:
		level = "error"
	}

	data, _ := NewJsonData([]byte(e.Data))

	return PrettyLogEntry{
		Level:      level,
		Project:    NullString{e.Project},
		DeliveryId: NullString{e.DeliveryId},
		PipeId:     NullString{e.PipeId},
		Message:    e.Message,
		Data:       data,
		Ts:         Timestamp{e.Ts},
	}
}

func LogEntries(les []logsDb.LogEntry) []PrettyLogEntry {
	entries := make([]PrettyLogEntry, len(les))
	for i, e := range les {
		entries[i] = LogEntry(e)
	}
	return entries
}
