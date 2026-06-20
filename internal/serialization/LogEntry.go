package serialization

import (
	"database/sql"
	"log/slog"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/logsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
)

func nullStringPtr(ns sql.NullString) *string {
	if !ns.Valid {
		return nil
	}
	return &ns.String
}

type PrettyLogEntry struct {
	Level      string    `json:"level"`
	Project    *string   `json:"project"`
	DeliveryID *string   `json:"deliveryId"`
	PipeID     *string   `json:"pipeId"`
	Message    string    `json:"message"`
	Data       JSONData  `json:"data"`
	TS         time.Time `json:"ts"`
}

func LogEntry(e logsdb.LogEntry) PrettyLogEntry {
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
		Project:    nullStringPtr(e.Project),
		DeliveryID: nullStringPtr(e.DeliveryID),
		PipeID:     nullStringPtr(e.PipeID),
		Message:    e.Message,
		Data:       data,
		TS:         e.TS,
	}
}

func LogEntries(les []logsdb.LogEntry) []PrettyLogEntry {
	entries := make([]PrettyLogEntry, len(les))
	for i, e := range les {
		entries[i] = LogEntry(e)
	}
	return entries
}

func LogEntriesPage(page models.PagedDB[logsdb.LogEntry]) models.Paged[PrettyLogEntry] {
	var result models.Paged[PrettyLogEntry]
	result.Items = LogEntries(page.Items)
	result.TotalCount = page.TotalCount
	return result
}
