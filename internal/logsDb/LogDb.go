package logsDb

import (
	"database/sql"
	_ "embed"
	"log/slog"
	"strings"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
)

type LogsDb struct {
	db *sqlx.DB
}

func New(dbFileName string) (*LogsDb, error) {
	if dbFileName == "" {
		return nil, nil
	}
	db := LogsDb{}
	pragmas := "?_journal_mode=WAL&_foreign_keys=1&_busy_timeout=5000&_cache_size=2000&_synchronous=NORMAL"
	d, err := sqlx.Open("sqlite3", dbFileName+pragmas)
	if err != nil {
		return nil, err
	}
	db.db = d
	err = db.open() // trying to open and migrate if necessasry the db
	if err != nil {
		return nil, err
	}
	return &db, nil
}

//go:embed Init.sql
var schema string

func (d LogsDb) open() error {
	var userVersion int
	err := d.db.Get(&userVersion, "PRAGMA user_version;")

	if err == nil && userVersion == 0 {
		_, err = d.db.Exec(schema)
	}

	return err
}

func (d *LogsDb) Close() (err error) {
	if d.db != nil {
		err = d.db.Close()
	}
	d.db = nil
	return err
}

func (d LogsDb) IsOpen() bool {
	return d.db != nil
}

type LogEntry struct {
	Id         int64          `db:"id"`
	Level      int            `db:"level"`
	Project    sql.NullString `db:"project"`
	DeliveryId sql.NullString `db:"delivery_id"`
	PipeId     sql.NullString `db:"pipe_id"`
	Message    string         `db:"message"`
	Data       string         `db:"data"`
	Ts         int64          `db:"ts"`
}

func (d LogsDb) CreateEntry(entry LogEntry) error {
	query := "INSERT INTO logs (level, project, delivery_id, pipe_id, message, data) VALUES (?, ?, ?, ?, ?, ?)"
	_, err := d.db.Exec(query, entry.Level, entry.Project, entry.DeliveryId, entry.PipeId, entry.Message, entry.Data)
	return err
}

const maxPageSize int = 200
const defaultPageSize int = 20

type GetEntryQuery struct {
	CursorId int64 `json:"cursorId"`
	CursorTs int64 `json:"cursorTs"`
	PageSize int   `json:"pageSize"`
}

func (d LogsDb) GetEntry(search GetEntryQuery) ([]LogEntry, error) {
	if search.PageSize <= 0 || search.PageSize > maxPageSize {
		search.PageSize = defaultPageSize
	}

	rows := []LogEntry{}
	query := `SELECT * from logs where (ts, id) > (?, ?) ORDER BY ts, id LIMIT ?`
	err := d.db.Select(&rows, query, search.CursorTs, search.CursorId, search.PageSize)
	return rows, err
}

type GetEntryFilteredQuery struct {
	GetEntryQuery
	Levels     []int  `json:"levels"`
	Project    string `json:"project"`
	DeliveryId string `json:"deliveryId"`
	PipeId     string `json:"pipeId"`
	Message    string `json:"message"`
}

func (d LogsDb) GetEntryFiltered(search GetEntryFilteredQuery) ([]LogEntry, error) {
	if search.PageSize <= 0 || search.PageSize > maxPageSize {
		search.PageSize = defaultPageSize
	}
	if search.Levels == nil || len(search.Levels) == 0 {
		search.Levels = make([]int, 4)
		search.Levels[0] = int(slog.LevelDebug)
		search.Levels[1] = int(slog.LevelInfo)
		search.Levels[2] = int(slog.LevelWarn)
		search.Levels[3] = int(slog.LevelError)
	}
	rows := []LogEntry{}

	var qb strings.Builder
	qb.WriteString("SELECT * from logs where (ts, id) > (?, ?)\n")

	args := make([]interface{}, 2)
	args[0] = search.CursorTs
	args[1] = search.CursorId

	if search.Levels != nil && len(search.Levels) > 0 {
		query, listArgs, err := sqlx.In("AND level IN (?)\n", search.Levels)
		if err != nil {
			return rows, err
		}
		query = d.db.Rebind(query)
		qb.WriteString(query)
		args = append(args, listArgs...)
	}

	if search.Project != "" {
		qb.WriteString("AND project LIKE ?\n")
		args = append(args, "%"+search.Project+"%")
	}

	if search.DeliveryId != "" {
		qb.WriteString("AND delivery_id LIKE ?\n")
		args = append(args, "%"+search.DeliveryId+"%")
	}

	if search.PipeId != "" {
		qb.WriteString("AND pipe_id LIKE ?\n")
		args = append(args, "%"+search.PipeId+"%")
	}

	if search.Message != "" {
		qb.WriteString("AND message LIKE ?\n")
		args = append(args, "%"+search.Message+"%")
	}

	qb.WriteString("ORDER BY ts, id LIMIT ?")
	args = append(args, search.PageSize)

	err := d.db.Select(&rows, qb.String(), args...)
	return rows, err
}
