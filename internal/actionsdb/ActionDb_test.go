package actionsdb_test

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/google/go-cmp/cmp"
	"github.com/google/go-cmp/cmp/cmpopts"
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
			Hash:       hash,
			Config:     actionJSON(t),
			CreatedAt:  time.Now(),
		}

		if diff := cmp.Diff(want, record, recordCmpOpts); diff != "" {
			t.Error(diff)
		}
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
			Hash:       hash,
			Config:     actionJSON(t),
			Error:      nil,
			CreatedAt:  time.Now(),
			EndedAt:    new(time.Now()),
		}

		if diff := cmp.Diff(want, record, recordCmpOpts); diff != "" {
			t.Error(diff)
		}
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
			Hash:       hash,
			Config:     actionJSON(t),
			Error:      actionErr,
			CreatedAt:  time.Now(),
			EndedAt:    new(time.Now()),
		}

		if diff := cmp.Diff(want, record, recordCmpOpts); diff != "" {
			t.Error(diff)
		}
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
			Hash:       hash,
			Config:     actionJSON(t),
			CreatedAt:  time.Now(),
		}

		if diff := cmp.Diff(want, record, recordCmpOpts); diff != "" {
			t.Error(diff)
		}
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

// recordCmpOpts compares records read back from the db: IDs are db-assigned,
// timestamps are set at write time and errors are restored from their message.
var recordCmpOpts = cmp.Options{
	cmpopts.IgnoreFields(actionsdb.PipeLineRecord{}, "ID"),
	cmpopts.EquateApproxTime(time.Minute),
	cmp.Comparer(func(a, b error) bool {
		if a == nil || b == nil {
			return a == b
		}
		return a.Error() == b.Error()
	}),
}

// actionJSON is the action config as it's stored in the db
func actionJSON(t *testing.T) json.RawMessage {
	t.Helper()
	data, err := json.Marshal(action)
	if err != nil {
		t.Fatalf("failed to marshal action: %s", err)
	}
	return data
}
