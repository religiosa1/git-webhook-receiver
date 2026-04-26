package serialization

import (
	"log/slog"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
)

type PrettyLogEntry struct {
	Level      string     `json:"level"`
	Project    NullString `json:"project"`
	DeliveryID NullString `json:"deliveryId"`
	PipeID     NullString `json:"pipeId"`
	Message    string     `json:"message"`
	Data       JSONData   `json:"data"`
	TS         Timestamp  `json:"ts"`
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

	data, _ := NewJSONData([]byte(e.Data))

	return PrettyLogEntry{
		Level:      level,
		Project:    NullString{e.Project},
		DeliveryID: NullString{e.DeliveryID},
		PipeID:     NullString{e.PipeID},
		Message:    e.Message,
		Data:       data,
		TS:         Timestamp{e.TS},
	}
}

func LogEntries(les []logsDb.LogEntry) []PrettyLogEntry {
	entries := make([]PrettyLogEntry, len(les))
	for i, e := range les {
		entries[i] = LogEntry(e)
	}
	return entries
}

func LogEntriesPage(page models.PagedDB[logsDb.LogEntry]) models.Paged[PrettyLogEntry] {
	var result models.Paged[PrettyLogEntry]
	result.Items = LogEntries(page.Items)
	result.TotalCount = page.TotalCount
	return result
}
