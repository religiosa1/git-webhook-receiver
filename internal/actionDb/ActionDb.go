package actiondb

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

type PipeLineRecord struct {
	Id         int64           `db:"id"`
	PipeId     string          `db:"pipe_id" json:"pipeId"`
	Project    string          `db:"project" json:"project"`
	DeliveryId string          `db:"delivery_id" json:"deliveryId"`
	Config     json.RawMessage `db:"config" json:"config"`
	Error      sql.NullString  `db:"error" json:"error,omitempty"`
	Output     sql.NullString  `db:"output" json:"output,omitempty"`
	CreatedAt  int64           `db:"created_at" json:"createdAt"`
	EndedAt    sql.NullInt64   `db:"ended_at" json:"endedAt,omitempty"`
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

func (d ActionDb) ListPipelineRecords(n int) ([]PipeLineRecord, error) {
	var records []PipeLineRecord
	query := `
SELECT * FROM (
	SELECT 
		id, pipe_id, project, delivery_id, error, created_at, ended_at 
	FROM 
		pipeline 
	ORDER BY 
		created_at DESC 
	LIMIT ?
) ORDER BY created_at ASC;`
	err := d.db.Select(&records, query, n)
	return records, err
}
