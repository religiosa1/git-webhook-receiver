package logsDb

import (
	"database/sql"
	_ "embed"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/models"
	sqlfilterbuilder "github.com/religiosa1/git-webhook-receiver/internal/sqlFilterBuilder"
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

type LogEntry struct {
	ID         int64          `db:"id"`
	Level      int            `db:"level"`
	Project    sql.NullString `db:"project"`
	DeliveryID sql.NullString `db:"delivery_id"`
	PipeID     sql.NullString `db:"pipe_id"`
	Message    string         `db:"message"`
	Data       string         `db:"data"`
	TS         int64          `db:"ts"`
}

func (d LogsDB) CreateEntry(entry LogEntry) error {
	query := "INSERT INTO logs (level, project, delivery_id, pipe_id, message, data) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := d.db.Exec(
		query,
		entry.Level,
		entry.Project,
		entry.DeliveryID,
		entry.PipeID,
		entry.Message,
		entry.Data,
	)
	return err
}

const (
	maxPageSize     int = 200
	defaultPageSize int = 20
)

var (
	ErrBadCursor       = errors.New("bad cursor")
	ErrCursorAndOffset = errors.New("cursor and offset cannot be supplied simultaneously")
)

type GetEntryFilteredQuery struct {
	Levels     []int  `json:"levels"`
	Project    string `json:"project"`
	DeliveryID string `json:"deliveryId"`
	PipeID     string `json:"pipeId"`
	Message    string `json:"message"`
	Offset     int    `json:"offset"`
	PageSize   int    `json:"pageSize"`
	Cursor     string `json:"cursor"`
}

func (d LogsDB) GetEntryFiltered(search GetEntryFilteredQuery) (models.PagedDB[LogEntry], error) {
	if search.PageSize <= 0 || search.PageSize > maxPageSize {
		search.PageSize = defaultPageSize
	}
	var result models.PagedDB[LogEntry]

	if search.Offset != 0 && search.Cursor != "" {
		return result, ErrCursorAndOffset
	}

	if len(search.Levels) == 0 {
		search.Levels = []int{
			int(slog.LevelDebug),
			int(slog.LevelInfo),
			int(slog.LevelWarn),
			int(slog.LevelError),
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
	args = append(args, search.PageSize+1)

	if search.Offset != 0 {
		qb.WriteString("OFFSET ?\n")
		args = append(args, search.Offset)
	}

	err = d.db.Select(&result.Items, qb.String(), args...)
	if err != nil {
		return result, err
	}
	result.TotalCount, err = d.CountEntries(search)
	if err != nil {
		return result, err
	}
	if len(result.Items) > search.PageSize {
		lastRow := result.Items[search.PageSize-1]
		cursorStr := paginationCursor{TS: lastRow.TS, ID: lastRow.ID}.String()
		result.Cursor = &cursorStr
		result.Items = result.Items[0:search.PageSize]
	}
	return result, err
}

func buildWhereClauses(search GetEntryFilteredQuery) *sqlfilterbuilder.Builder {
	fb := sqlfilterbuilder.New()
	// TODO: do we really need like columns here as well?
	fb.AddLikeFilter("project", search.Project)
	fb.AddLikeFilter("delivery_id", search.DeliveryID)
	fb.AddLikeFilter("pipe_id", search.PipeID)

	// that's probably the only place where we really need LIKE
	fb.AddLikeFilter("message", search.Message)

	fb.AddInFilter("level", search.Levels)

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

func ParseLogLevel(level string) (int, error) {
	switch level {
	case "debug":
		return int(slog.LevelDebug), nil
	case "info":
		return int(slog.LevelInfo), nil
	case "warn":
		return int(slog.LevelWarn), nil
	case "error":
		return int(slog.LevelError), nil
	default:
		return 0, fmt.Errorf("unknown log level '%s'", level)
	}
}
