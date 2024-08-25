package logger

import (
	"context"
	"log/slog"

	"github.com/religiosa1/webhook-receiver/internal/logsDb"
)

type SlogSqlite struct {
	db      *logsDb.LogsDb
	attrs   []slog.Attr
	leveler slog.Leveler
	group   string
}

func NewDBLogger(db *logsDb.LogsDb, opts *slog.HandlerOptions) *SlogSqlite {
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

	// TODO parse args and populate logentry with them

	dbRecord := logsDb.LogEntry{
		Level: int(record.Level),
		// Project    sql.NullString `db:"project"`
		// DeliveryId sql.NullString `db:"delivery_id"`
		// PipeId     sql.NullString `db:"pipe_id"`
		Message: record.Message,
		// Data       string         `db:"data"`
		Ts: record.Time.UTC().Unix(),
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

// TODO why we even need this?..
func (logger *SlogSqlite) WithGroup(name string) slog.Handler {
	return &SlogSqlite{
		db:      logger.db,
		leveler: logger.leveler,
		attrs:   logger.attrs,
		group:   name,
	}
}
