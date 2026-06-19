package actionsdb_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/oklog/ulid/v2"
	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

const defaultMaxActionsStored = 100

const (
	pipeID      = "123"
	projectName = "testProj"
	deliveryID  = "321"
	hash        = "6789"
)

var action = config.Action{
	On:     "push",
	Branch: "main",
	Cwd:    "/var/www",
	User:   "www-data",
	Script: "whoami",
}

func TestActionDb(t *testing.T) {
	t.Run("successfully creates a db", func(t *testing.T) {
		_, err := actionsdb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Failed to create a db: %s", err)
		}
	})
	t.Run("creates a record", func(t *testing.T) {
		db, err := actionsdb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeID)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		want := actionsdb.PipeLineRecord{
			PipeID:     pipeID,
			Project:    projectName,
			DeliveryID: deliveryID,
			Hash:       sql.NullString{Valid: true, String: hash},
		}

		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("Close successful action", func(t *testing.T) {
		actionOutput := "test output"
		db, err := actionsdb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeID, nil, []byte(actionOutput))
		if err != nil {
			t.Fatalf("Unable to close a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeID)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		want := actionsdb.PipeLineRecord{
			PipeID:     pipeID,
			Project:    projectName,
			DeliveryID: deliveryID,
			Hash:       sql.NullString{Valid: true, String: hash},
			Error:      sql.NullString{Valid: false},
			EndedAt:    sql.NullTime{Valid: true},
		}
		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("Close errored action", func(t *testing.T) {
		actionErr := errors.New("some error blah blah")
		actionOutput := "test output"
		db, err := actionsdb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeID, actionErr, []byte(actionOutput))
		if err != nil {
			t.Fatalf("Unable to close a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeID)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		want := actionsdb.PipeLineRecord{
			PipeID:     pipeID,
			Project:    projectName,
			DeliveryID: deliveryID,
			Hash:       sql.NullString{Valid: true, String: hash},
			Error:      sql.NullString{Valid: true, String: actionErr.Error()},
			EndedAt:    sql.NullTime{Valid: true},
		}
		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("An action can only be closed once", func(t *testing.T) {
		db, err := actionsdb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeID, nil, []byte(""))
		if err != nil {
			t.Fatalf("Unable to close a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeID, nil, []byte(""))
		if err == nil {
			t.Errorf("Repeated closing of an action was supposed to end with an error, but it didn't!")
		}
	})

	t.Run("db keeps data persistently", func(t *testing.T) {
		tmpdir := t.TempDir()
		tmpfile, err := os.CreateTemp(tmpdir, "*.sqlite3")
		if err != nil {
			t.Fatalf("Unable to create a tempfile for db: %s", err)
		}
		defer func() {
			_ = tmpfile.Close()
		}()
		db, err := actionsdb.New(tmpfile.Name(), defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a record: %s", err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Unable to close the db: %s", err)
		}

		db2, err := actionsdb.New(tmpfile.Name(), defaultMaxActionsStored)
		if err != nil {
			t.Fatalf("Unable to open the db for the second time: %s", err)
		}

		record, err := db2.GetPipelineRecord(pipeID)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		want := actionsdb.PipeLineRecord{
			PipeID:     pipeID,
			Project:    projectName,
			DeliveryID: deliveryID,
			Hash:       sql.NullString{Valid: true, String: hash},
		}

		compareRecord(t, want, record)
		compareAction(t, action, record)
	})
}

func TestAutoRemoval(t *testing.T) {
	createRecord := func(t *testing.T, db *actionsdb.ActionDB) string {
		t.Helper()
		pipeID := ulid.Make().String()
		err := db.CreateRecord(pipeID, projectName, deliveryID, hash, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeID, nil, []byte("test"))
		if err != nil {
			t.Errorf("Unable to close a pipeline record: %s", err)
		}
		return pipeID
	}

	countRecords := func(t *testing.T, db *actionsdb.ActionDB) int {
		count, err := db.CountPipelineRecords(actionsdb.ListPipelineRecordsQuery{})
		if err != nil {
			t.Errorf("Failed to count pipelines: %s", err)
		}
		return count
	}

	t.Run("removes old record", func(t *testing.T) {
		const maxRecords = 3
		db, err := actionsdb.New(":memory:", maxRecords)
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}
		for range maxRecords {
			_ = createRecord(t, db)
		}

		var lastPipeID string
		for range maxRecords {
			lastPipeID = createRecord(t, db)
		}

		if got := countRecords(t, db); got != maxRecords {
			t.Errorf("Unexpected amount of records after auto-removal; want %d, got %d", maxRecords, got)
		}

		_, err = db.GetPipelineRecord(lastPipeID)
		if err != nil {
			t.Errorf("Unable to retrieve last pipeline: %s", err)
		}
	})

	// "does nothing tests"
	cases := []struct {
		name   string
		amount int
	}{
		{"does nothing on negative config values", -1},
		// This is an improbable edge case, as 0 should be coerced to the config.DefaultMaxActionsStored value,
		// so ActionDB shouldn't really see 0 values on its input
		{"does nothing on 0 config values", 0},
	}
	for _, tt := range cases {
		t.Run(tt.name, func(t *testing.T) {
			// hitting over the default limit
			const nRecords = config.DefaultMaxActionsStored + 5
			db, err := actionsdb.New(":memory:", tt.amount)
			if err != nil {
				t.Fatalf("Unable to create a db: %s", err)
			}
			for range nRecords {
				_ = createRecord(t, db)
			}
			if got := countRecords(t, db); got != nRecords {
				t.Errorf("Unexpected amount of records; want %d, got %d", nRecords, got)
			}
		})
	}
}

func compareAction(t *testing.T, action config.Action, record actionsdb.PipeLineRecord) {
	t.Helper()

	var recordConfig config.Action
	err := json.Unmarshal(record.Config, &recordConfig)
	if err != nil {
		t.Fatalf("failed to unmarshal record config: %v, JSON: %s", err, string(record.Config))
		return
	}

	if !reflect.DeepEqual(action, recordConfig) {
		t.Errorf("record config does not match, want %v, got %v", action, recordConfig)
	}
}

func compareRecord(t *testing.T, want actionsdb.PipeLineRecord, got actionsdb.PipeLineRecord) {
	t.Helper()

	if want.PipeID != got.PipeID {
		t.Errorf("Bad pipeId: want %s, got %s,", want.PipeID, got.PipeID)
	}
	if want.Project != got.Project {
		t.Errorf("Bad project: want %s, got %s,", want.Project, got.Project)
	}
	if want.DeliveryID != got.DeliveryID {
		t.Errorf("Bad deliveryId, want %s, got %s", want.DeliveryID, got.DeliveryID)
	}
	if want.Hash != got.Hash {
		t.Errorf("Bad hash, want %v, got %v", want.Hash, got.Hash)
	}
	if want.Error != got.Error {
		t.Errorf("Unexpected error value in created record: want %v, got %v", want.Error, got.Error)
	}
	if got.CreatedAt.IsZero() {
		t.Errorf("Unexpected empty created date: want %s, got %s", want.CreatedAt, got.CreatedAt)
	}
	if want.EndedAt.Valid != got.EndedAt.Valid {
		t.Errorf("Unexpected emptiness of ended date: want %t, got %t", want.EndedAt.Valid, got.EndedAt.Valid)
	}
}
