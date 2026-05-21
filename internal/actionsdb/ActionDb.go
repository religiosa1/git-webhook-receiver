package actionsdb

import (
	"database/sql"
	_ "embed"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jmoiron/sqlx"
	_ "github.com/mattn/go-sqlite3"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

var (
	ErrCursorAndOffset = errors.New("cursor and offset pagination supplied simultaneously")
	ErrBadCursor       = errors.New("bad cursor")
)

// output is also stored in a row, but it's only fetched via a separate query
type pipelineRecordDTO struct {
	ID         int64           `db:"id"`
	PipeID     string          `db:"pipe_id"`
	Project    string          `db:"project"`
	DeliveryID string          `db:"delivery_id"`
	Config     json.RawMessage `db:"config"`
	Error      sql.NullString  `db:"error"`
	CreatedAt  int64           `db:"created_at"`
	EndedAt    sql.NullInt64   `db:"ended_at"`
}

func (r pipelineRecordDTO) ToModel() PipeLineRecord {
	return PipeLineRecord{
		ID:         r.ID,
		PipeID:     r.PipeID,
		Project:    r.Project,
		DeliveryID: r.DeliveryID,
		Config:     r.Config,
		Error:      r.Error,
		CreatedAt:  time.UnixMilli(r.CreatedAt).UTC(),
		EndedAt: sql.NullTime{
			Valid: r.EndedAt.Valid,
			Time:  time.UnixMilli(r.EndedAt.Int64).UTC(),
		},
	}
}

type PipeLineRecord struct {
	ID         int64
	PipeID     string
	Project    string
	DeliveryID string
	Config     json.RawMessage
	Error      sql.NullString
	CreatedAt  time.Time
	EndedAt    sql.NullTime
}

type PipeLineConfigSummary struct {
	Branch string `json:"branch"`
	On     string `json:"on"`
}

func (r PipeLineRecord) ParseConfigSummary() (PipeLineConfigSummary, error) {
	var summary PipeLineConfigSummary
	err := json.Unmarshal(r.Config, &summary)
	return summary, err
}

type ActionDB struct {
	db *sqlx.DB
	// maxActions is max count of actions to store, before truncating older ones
	maxActions int
}

func New(dbFileName string, maxActions int) (*ActionDB, error) {
	if dbFileName == "" {
		return nil, nil
	}
	db := ActionDB{maxActions: maxActions}
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

// SweepStaleRecords moves all pending pipeline records to errored state; to
// be called during a service startup, to cleanup records that were left stale
// after a non-graceful shutdown
func (d ActionDB) SweepStaleRecords() (int64, error) {
	const stalePipelineError = "pipeline was killed abruptly during a server crash"
	query := `UPDATE pipelines SET error = ?, ended_at = ? WHERE ended_at IS NULL AND error IS NULL`

	result, err := d.db.Exec(query, stalePipelineError, time.Now().UTC().UnixMilli())
	if err != nil {
		return 0, fmt.Errorf("error while updating stale pipeline record: %w", err)
	}
	rowsAffected, err := result.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("error while retrieving the amount of stale records updated: %w", err)
	}
	return rowsAffected, nil
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

	query := `INSERT INTO pipelines (pipe_id, project, delivery_id, config) VALUES (?, ?, ?, ?)`
	_, err = tx.Exec(query, pipeID, project, deliveryID, configJSON)
	if err != nil {
		_ = tx.Rollback()
		return err
	}

	// auto-removal of records above max actions config value
	if d.maxActions > 0 {
		autoRemoveQuery := `
DELETE FROM pipelines WHERE pipe_id IN (
		SELECT pipe_id FROM pipelines ORDER BY created_at DESC LIMIT -1 OFFSET ?
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

func (d ActionDB) CloseRecord(pipeID string, actionErr error, output []byte) error {
	var actionErrValue sql.NullString

	if actionErr == nil {
		actionErrValue.Valid = false
	} else {
		actionErrValue.Valid = true
		actionErrValue.String = actionErr.Error()
	}

	query := `UPDATE pipelines SET error = ?, output = ?, ended_at = ? WHERE pipe_id = ? AND ended_at IS NULL;`
	result, err := d.db.Exec(query, actionErrValue, output, time.Now().UTC().UnixMilli(), pipeID)
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

const recordColumns = "id, pipe_id, project, delivery_id, config, error, created_at, ended_at"

func (d ActionDB) GetPipelineRecord(pipeID string) (PipeLineRecord, error) {
	var record pipelineRecordDTO
	err := d.db.Get(
		&record,
		"SELECT "+recordColumns+" FROM pipelines WHERE pipe_id=?;",
		pipeID,
	)
	return record.ToModel(), err
}

func (d ActionDB) GetLastPipelineRecord() (PipeLineRecord, error) {
	var entry pipelineRecordDTO
	err := d.db.Get(
		&entry,
		"SELECT "+recordColumns+" FROM pipelines ORDER BY created_at DESC LIMIT 1;",
	)
	return entry.ToModel(), err
}

type output struct {
	Output sql.NullString `db:"output"`
}

func (d ActionDB) GetPipelineOutput(pipeID string) ([]byte, error) {
	var record output
	err := d.db.Get(
		&record, `SELECT output FROM pipelines WHERE pipe_id=?;`,
		pipeID,
	)
	if err != nil {
		return nil, err
	}
	return []byte(record.Output.String), nil
}

func (d ActionDB) GetLastPipelineOutput() ([]byte, error) {
	var record output
	err := d.db.Get(
		&record, `SELECT output FROM pipelines ORDER BY created_at DESC LIMIT 1;`,
	)
	if err != nil {
		return nil, err
	}
	return []byte(record.Output.String), nil
}
