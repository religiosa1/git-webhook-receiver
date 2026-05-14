package logsdb

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
	"github.com/religiosa1/git-webhook-receiver/internal/sqlfilterbuilder"
)

type LogsDB struct {
	db *sqlx.DB
}

func New(dbFileName string) (*LogsDB, error) {
	if dbFileName == "" {
		return nil, nil
	}
	db := LogsDB{}
	pragmas := "?_journal_mode=WAL&_foreign_keys=1&_busy_timeout=5000&_cache_size=2000&_synchronous=NORMAL"
	d, err := sqlx.Open("sqlite3", dbFileName+pragmas)
	if err != nil {
		return nil, err
	}
	db.db = d
	err = db.open() // trying to open and migrate if necessary the db
	if err != nil {
		return nil, err
	}
	return &db, nil
}

//go:embed Init.sql
var schema string

func (d LogsDB) open() error {
	var userVersion int
	err := d.db.Get(&userVersion, "PRAGMA user_version;")

	if err == nil && userVersion == 0 {
		_, err = d.db.Exec(schema)
	}

	return err
}

func (d *LogsDB) Close() (err error) {
	if d.db != nil {
		err = d.db.Close()
	}
	d.db = nil
	return err
}

func (d LogsDB) IsOpen() bool {
	return d.db != nil
}

type logEntryDto struct {
	ID         int64          `db:"id"`
	Level      int            `db:"level"`
	Project    sql.NullString `db:"project"`
	DeliveryID sql.NullString `db:"delivery_id"`
	PipeID     sql.NullString `db:"pipe_id"`
	Message    string         `db:"message"`
	Data       string         `db:"data"`
	TS         int64          `db:"ts"`
}

type LogEntry struct {
	ID         int64
	Level      slog.Level
	Project    sql.NullString
	DeliveryID sql.NullString
	PipeID     sql.NullString
	Message    string
	Data       string
	TS         time.Time
}

func (e logEntryDto) ToModel() LogEntry {
	return LogEntry{
		ID:         e.ID,
		Level:      slog.Level(e.Level),
		Project:    e.Project,
		DeliveryID: e.DeliveryID,
		PipeID:     e.PipeID,
		Message:    e.Message,
		Data:       e.Data,
		TS:         time.UnixMilli(e.TS).UTC(),
	}
}

func (d LogsDB) CreateEntry(entry LogEntry) error {
	query := "INSERT INTO logs (level, project, delivery_id, pipe_id, message, data, ts) VALUES (?, ?, ?, ?, ?, ?, ?)"
	_, err := d.db.Exec(
		query,
		entry.Level,
		entry.Project,
		entry.DeliveryID,
		entry.PipeID,
		entry.Message,
		entry.Data,
		entry.TS.UnixMilli(),
	)
	return err
}

const (
	defaultPageSize int = 20
)

var (
	ErrBadCursor       = errors.New("bad cursor")
	ErrCursorAndOffset = errors.New("cursor and offset cannot be supplied simultaneously")
)

type GetEntryFilteredQuery struct {
	Levels     []slog.Level `json:"levels"`
	Project    string       `json:"project"`
	DeliveryID string       `json:"deliveryId"`
	PipeID     string       `json:"pipeId"`
	Message    string       `json:"message"`
	Offset     int          `json:"offset"`
	Limit      int          `json:"limit"`
	Cursor     string       `json:"cursor"`
}

func (d LogsDB) GetEntryFiltered(search GetEntryFilteredQuery) (models.PagedDB[LogEntry], error) {
	if search.Limit <= 0 {
		search.Limit = defaultPageSize
	}
	var result models.PagedDB[LogEntry]

	if search.Offset != 0 && search.Cursor != "" {
		return result, ErrCursorAndOffset
	}

	if len(search.Levels) == 0 {
		search.Levels = []slog.Level{
			slog.LevelDebug,
			slog.LevelInfo,
			slog.LevelWarn,
			slog.LevelError,
		}
	}

	var qb strings.Builder
	qb.WriteString("SELECT * from logs\n")
	args := []any{}

	fb := buildWhereClauses(search)

	cursor, err := newCursorFromStr(search.Cursor)
	if err != nil {
		return result, err
	}
	if cursor != nil {
		fb.AddParamFilter("(ts, id) < (?, ?)", cursor.TS, cursor.ID)
	}
	if fb.HasFilters() {
		qb.WriteString("WHERE\n")
		qb.WriteString(fb.String())
		args = fb.Args()
	}

	qb.WriteString("ORDER BY ts DESC, id DESC\n")
	qb.WriteString("LIMIT ?\n")
	args = append(args, search.Limit+1)

	if search.Offset != 0 {
		qb.WriteString("OFFSET ?\n")
		args = append(args, search.Offset)
	}

	var items []logEntryDto
	err = d.db.Select(&items, qb.String(), args...)
	if err != nil {
		return result, err
	}
	result.TotalCount, err = d.CountEntries(search)
	if err != nil {
		return result, err
	}
	if len(items) > search.Limit {
		lastRow := items[search.Limit-1]
		cursorStr := paginationCursor{TS: lastRow.TS, ID: lastRow.ID}.String()
		result.Cursor = &cursorStr
		items = items[0:search.Limit]
	}
	result.Items = make([]LogEntry, len(items))
	for i, item := range items {
		result.Items[i] = item.ToModel()
	}
	return result, err
}

func buildWhereClauses(search GetEntryFilteredQuery) *sqlfilterbuilder.Builder {
	fb := sqlfilterbuilder.New()
	fb.AddEqFilter("project", search.Project)
	fb.AddEqFilter("delivery_id", search.DeliveryID)
	fb.AddEqFilter("pipe_id", search.PipeID)

	fb.AddLikeFilter("message", search.Message)
	levels := make([]int, len(search.Levels))
	for i, l := range search.Levels {
		levels[i] = int(l)
	}
	fb.AddInFilter("level", levels)

	return fb
}

func (d LogsDB) CountEntries(search GetEntryFilteredQuery) (int, error) {
	var qb strings.Builder
	qb.WriteString("SELECT count(*) from logs\n")
	args := []any{}

	fb := buildWhereClauses(search)

	if fb.HasFilters() {
		qb.WriteString("WHERE\n")
		qb.WriteString(fb.String())
		args = append(args, fb.Args()...)
	}
	row := d.db.QueryRow(qb.String(), args...)
	var count int
	err := row.Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}

func ParseLogLevel(level string) (slog.Level, error) {
	switch level {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return 0, fmt.Errorf("unknown log level %q", level)
	}
}
