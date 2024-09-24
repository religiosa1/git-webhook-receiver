package actiondb

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type PipeLineRecord struct {
	Id         int64           `db:"id"`
	PipeId     string          `db:"pipe_id"`
	Project    string          `db:"project"`
	DeliveryId string          `db:"delivery_id"`
	Config     json.RawMessage `db:"config"`
	Error      sql.NullString  `db:"error"`
	Output     sql.NullString  `db:"output"`
	CreatedAt  int64           `db:"created_at"`
	EndedAt    sql.NullInt64   `db:"ended_at"`
}

type ActionDb struct {
	db *sqlx.DB
}

func New(dbFileName string) (*ActionDb, error) {
	if dbFileName == "" {
		return nil, nil
	}
	db := ActionDb{}
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

func (d ActionDb) open() error {
	var userVersion int
	err := d.db.Get(&userVersion, "PRAGMA user_version;")

	if err == nil && userVersion == 0 {
		_, err = d.db.Exec(schema)
	}

	return err
}

func (d ActionDb) Close() error {
	return d.db.Close()
}

func (d ActionDb) CreateRecord(pipeId string, project string, deliveryId string, conf config.Action) error {
	configJson, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	query := `INSERT INTO pipeline (pipe_id, project, delivery_id, config) VALUES (?, ?, ?, ?)`
	_, err = d.db.Exec(query, pipeId, project, deliveryId, configJson)
	return err
}

func (d ActionDb) CloseRecord(pipeId string, actionErr error, output string) error {
	var actionErrValue sql.NullString

	if actionErr == nil {
		actionErrValue.Valid = false
	} else {
		actionErrValue.Valid = true
		actionErrValue.String = actionErr.Error()
	}

	query := `UPDATE pipeline SET error = ?, output = ?, ended_at = ? WHERE pipe_id = ? AND ended_at IS NULL;`
	result, err := d.db.Exec(query, actionErrValue, output, time.Now().UTC().Unix(), pipeId)
	if err != nil {
		return fmt.Errorf("error while updating pipeline record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error while determining result of the pipeline record updagte: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("unable to find the wor to update: pipeId = %s", pipeId)
	}
	return err
}

func (d ActionDb) GetPipelineRecord(pipeId string) (PipeLineRecord, error) {
	var record PipeLineRecord
	if pipeId == "" {
		err := d.db.Get(&record, "SELECT * FROM pipeline ORDER BY created_at DESC LIMIT 1;")
		return record, err
	}
	err := d.db.Get(&record, "SELECT * FROM pipeline WHERE pipe_id=?;", pipeId)
	return record, err
}

type PipeStatus int

const (
	PipeStatusAny     PipeStatus = 0
	PipeStatusOk      PipeStatus = 1
	PipeStatusError   PipeStatus = 2
	PipeStatusPending PipeStatus = 3
)

type ListPipelineRecordsQuery struct {
	Offset     int
	Limit      int
	Status     PipeStatus
	Project    string
	DeliveryId string
}

const maxPageSize int = 200

func (d ActionDb) ListPipelineRecords(search ListPipelineRecordsQuery) ([]PipeLineRecord, error) {
	if search.Limit <= 0 || search.Limit > maxPageSize {
		search.Limit = 20
	}

	var qb strings.Builder
	args := make([]interface{}, 0)

	qb.WriteString(`
SELECT * FROM (
SELECT 
	id, pipe_id, project, delivery_id, config, error, created_at, ended_at 
FROM 
	pipeline
`)

	fj := filterJoiner{}

	fj.AddLikeFilter(search.DeliveryId, "delivery_id")
	fj.AddLikeFilter(search.Project, "project")

	switch search.Status {
	case PipeStatusOk:
		fj.AddFilter("(ended_at IS NOT NULL AND (error IS NULL OR error = ''))\n")
	case PipeStatusError:
		fj.AddFilter("(ended_at IS NOT NULL AND (error IS NOT NULL AND error <> ''))\n")
	case PipeStatusPending:
		fj.AddFilter("ended_at IS NULL\n")
	}

	if fj.HasFilters {
		qb.WriteString("WHERE\n")
		qb.WriteString(fj.String())
		args = append(args, fj.Args()...)
	}

	qb.WriteString("ORDER BY created_at DESC\n")
	qb.WriteString("LIMIT ?\n")
	args = append(args, search.Limit)

	if search.Offset != 0 {
		qb.WriteString("OFFSET ?\n")
		args = append(args, search.Offset)
	}

	qb.WriteString(") ORDER BY created_at ASC;")

	var records []PipeLineRecord
	err := d.db.Select(&records, qb.String(), args...)
	return records, err
}

func ParsePipelineStatus(status string) (PipeStatus, error) {
	switch status {
	case "ok":
		return PipeStatusOk, nil
	case "error":
		return PipeStatusError, nil
	case "pending":
		return PipeStatusPending, nil
	case "any":
		return PipeStatusAny, nil
	default:
		return PipeStatusAny, fmt.Errorf("unknown pipe status: '%s'", status)
	}
}
