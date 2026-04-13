package logger

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
)

type SlogSqlite struct {
	db      *logsDb.LogsDB
	attrs   []slog.Attr
	leveler slog.Leveler
	group   string
}

func NewDBLogger(db *logsDb.LogsDB, opts *slog.HandlerOptions) *SlogSqlite {
	return &SlogSqlite{
		db:      db,
		leveler: opts.Level,
	}
}

func (logger *SlogSqlite) Enabled(_ context.Context, level slog.Level) bool {
	return logger.db.IsOpen() && level >= logger.leveler.Level()
}

func (logger *SlogSqlite) Handle(ctx context.Context, record slog.Record) error {
	if !logger.Enabled(ctx, record.Level) {
		return nil
	}

	dbRecord := logsDb.LogEntry{
		Level:   int(record.Level),
		Message: record.Message,
		TS:      record.Time.UTC().Unix(),
	}

	dataObj := make(map[string]any)

	procAttr := func(a slog.Attr) bool {
		switch a.Key {
		case "project":
			dbRecord.Project = sql.NullString{Valid: true, String: a.Value.String()}
		case "deliveryId":
			dbRecord.DeliveryID = sql.NullString{Valid: true, String: a.Value.String()}
		case "pipeId":
			dbRecord.PipeID = sql.NullString{Valid: true, String: a.Value.String()}
		default:
			dataObj[a.Key] = a.Value.Any()
		}
		return true
	}

	for _, a := range logger.attrs {
		procAttr(a)
	}
	record.Attrs(procAttr)

	if len(dataObj) > 0 {
		bytes, err := json.Marshal(dataObj)
		if err != nil {
			return fmt.Errorf("error while encoding record attrs to JSON: %w", err)
		}
		dbRecord.Data = string(bytes)
	}

	return logger.db.CreateEntry(dbRecord)
}

func (logger *SlogSqlite) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &SlogSqlite{
		db:      logger.db,
		leveler: logger.leveler,
		attrs:   append(logger.attrs, attrs...),
		group:   logger.group,
	}
}

// WithGroup is part of slog.Handler interface
func (logger *SlogSqlite) WithGroup(name string) slog.Handler {
	return &SlogSqlite{
		db:      logger.db,
		leveler: logger.leveler,
		attrs:   logger.attrs,
		group:   name,
	}
}
