package actiondb

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

var (
	ErrCursorAndOffset = errors.New("cursor and offset pagination supplied simultaneously")
	ErrBadCursor       = errors.New("bad cursor")
)

type PipeLineRecord struct {
	ID         int64           `db:"id"`
	PipeID     string          `db:"pipe_id"`
	Project    string          `db:"project"`
	DeliveryID string          `db:"delivery_id"`
	Config     json.RawMessage `db:"config"`
	Error      sql.NullString  `db:"error"`
	Output     sql.NullString  `db:"output"`
	CreatedAt  int64           `db:"created_at"`
	EndedAt    sql.NullInt64   `db:"ended_at"`
}

type ActionDB struct {
	db             *sqlx.DB
	maxActions     int
	maxOutputBytes int
}

func New(dbFileName string, maxActions int, maxOutputBytes int) (*ActionDB, error) {
	if dbFileName == "" {
		return nil, nil
	}
	db := ActionDB{maxActions: maxActions, maxOutputBytes: maxOutputBytes}
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

func (d ActionDB) open() error {
	var userVersion int
	err := d.db.Get(&userVersion, "PRAGMA user_version;")

	if err == nil && userVersion == 0 {
		_, err = d.db.Exec(schema)
	}

	return err
}

func (d ActionDB) Close() error {
	return d.db.Close()
}

func (d ActionDB) CreateRecord(pipeID string, project string, deliveryID string, conf config.Action) error {
	configJSON, err := json.Marshal(conf)
	if err != nil {
		return err
	}
	tx, err := d.db.Begin()
	if err != nil {
		return err
	}

	query := `INSERT INTO pipeline (pipe_id, project, delivery_id, config) VALUES (?, ?, ?, ?)`
	_, err = tx.Exec(query, pipeID, project, deliveryID, configJSON)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// auto-removal of records above max actions config value
	if d.maxActions > 0 {
		autoRemoveQuery := `
DELETE FROM pipeline WHERE pipe_id IN (
		SELECT pipe_id FROM pipeline ORDER BY created_at DESC LIMIT -1 OFFSET ?
)`
		_, err = tx.Exec(autoRemoveQuery, d.maxActions)
		if err != nil {
			_ = tx.Rollback()
			return err
		}
	}
	err = tx.Commit()
	return err
}

func (d ActionDB) CloseRecord(pipeID string, actionErr error, output io.Reader) error {
	var actionErrValue sql.NullString

	if actionErr == nil {
		actionErrValue.Valid = false
	} else {
		actionErrValue.Valid = true
		actionErrValue.String = actionErr.Error()
	}

	outputStr, err := readOutput(output, d.maxOutputBytes)
	if err != nil {
		return fmt.Errorf("error reading action output: %w", err)
	}

	query := `UPDATE pipeline SET error = ?, output = ?, ended_at = ? WHERE pipe_id = ? AND ended_at IS NULL;`
	result, err := d.db.Exec(query, actionErrValue, outputStr, time.Now().UTC().Unix(), pipeID)
	if err != nil {
		return fmt.Errorf("error while updating pipeline record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("error while determining result of the pipeline record update: %w", err)
	}
	if rowsAffected == 0 {
		return fmt.Errorf("unable to find the row to update: pipeId = %s", pipeID)
	}
	return err
}

func readOutput(r io.Reader, maxBytes int) (string, error) {
	if maxBytes <= 0 {
		buf, err := io.ReadAll(r)
		return string(buf), err
	}
	buf, err := io.ReadAll(io.LimitReader(r, int64(maxBytes)+1))
	if err != nil {
		return "", err
	}
	if len(buf) > maxBytes {
		return string(buf[:maxBytes]) + fmt.Sprintf("\n[output truncated at %d bytes]", maxBytes), nil
	}
	return string(buf), nil
}

func (d ActionDB) GetPipelineRecord(pipeID string) (PipeLineRecord, error) {
	var record PipeLineRecord
	if pipeID == "" {
		err := d.db.Get(&record, "SELECT * FROM pipeline ORDER BY created_at DESC LIMIT 1;")
		return record, err
	}
	err := d.db.Get(&record, "SELECT * FROM pipeline WHERE pipe_id=?;", pipeID)
	return record, err
}
