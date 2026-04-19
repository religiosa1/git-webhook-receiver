package actiondb_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	"github.com/oklog/ulid/v2"
	actiondb "github.com/religiosa1/git-webhook-receiver/internal/actionDb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

const defaultMaxActionsStored = 100

const (
	pipeId      = "123"
	projectName = "testProj"
	deliveryId  = "321"
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
		_, err := actiondb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Error((err))
		}
	})
	t.Run("creates a record", func(t *testing.T) {
		db, err := actiondb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			Project:    projectName,
			DeliveryId: deliveryId,
		}

		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("Close successful action", func(t *testing.T) {
		actionOutput := "test output"
		db, err := actiondb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, actionOutput)
		if err != nil {
			t.Errorf("Unable to close a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			Project:    projectName,
			DeliveryId: deliveryId,
			Output:     sql.NullString{Valid: true, String: actionOutput},
			Error:      sql.NullString{Valid: false},
			EndedAt:    sql.NullInt64{Valid: true},
		}
		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("Close errored action", func(t *testing.T) {
		actionErr := errors.New("some error blah blah")
		actionOutput := "test output"
		db, err := actiondb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeId, actionErr, actionOutput)
		if err != nil {
			t.Errorf("Unable to close a pipeline record: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			Project:    projectName,
			DeliveryId: deliveryId,
			Output:     sql.NullString{Valid: true, String: actionOutput},
			Error:      sql.NullString{Valid: true, String: actionErr.Error()},
			EndedAt:    sql.NullInt64{Valid: true},
		}
		compareRecord(t, want, record)
		compareAction(t, action, record)
	})

	t.Run("An action can only be closed once", func(t *testing.T) {
		db, err := actiondb.New(":memory:", defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, "")
		if err != nil {
			t.Errorf("Unable to close a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, "")
		if err == nil {
			t.Errorf("Repeated closing of an action was supposed to end with an error, but it didn't!")
		}
	})

	t.Run("db keeps data persistently", func(t *testing.T) {
		tmpdir := t.TempDir()
		tmpfile, err := os.CreateTemp(tmpdir, "*.sqlite3")
		if err != nil {
			t.Errorf("Unable to create a tempfile for db: %s", err)
		}
		defer func() {
			_ = tmpfile.Close()
		}()
		db, err := actiondb.New(tmpfile.Name(), defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a record: %s", err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Unable to close the db: %s", err)
		}

		db2, err := actiondb.New(tmpfile.Name(), defaultMaxActionsStored)
		if err != nil {
			t.Errorf("Unable to open the db for the second time: %s", err)
		}

		record, err := db2.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			Project:    projectName,
			DeliveryId: deliveryId,
		}

		compareRecord(t, want, record)
		compareAction(t, action, record)
	})
}

func TestAutoRemoval(t *testing.T) {
	createRecord := func(t *testing.T, db *actiondb.ActionDb) string {
		t.Helper()
		pipeId := ulid.Make().String()
		err := db.CreateRecord(pipeId, projectName, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline record: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, "test")
		if err != nil {
			t.Errorf("Unable to close a pipeline record: %s", err)
		}
		return pipeId
	}

	countRecords := func(t *testing.T, db *actiondb.ActionDb) int {
		count, err := db.CountPipelineRecords(actiondb.ListPipelineRecordsQuery{})
		if err != nil {
			t.Errorf("Failed to count pipelines: %s", err)
		}
		return count
	}

	t.Run("removes old record", func(t *testing.T) {
		const maxRecords = 3
		db, err := actiondb.New(":memory:", maxRecords)
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}
		for i := 0; i < maxRecords; i++ {
			_ = createRecord(t, db)
		}

		var lastPipeId string
		for i := 0; i < maxRecords; i++ {
			lastPipeId = createRecord(t, db)
		}

		if got := countRecords(t, db); got != maxRecords {
			t.Errorf("Unexpected amount of records after auto-removal; want %d, got %d", maxRecords, got)
		}

		_, err = db.GetPipelineRecord(lastPipeId)
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
			db, err := actiondb.New(":memory:", tt.amount)
			if err != nil {
				t.Errorf("Unable to create a db: %s", err)
			}
			for i := 0; i < nRecords; i++ {
				_ = createRecord(t, db)
			}
			if got := countRecords(t, db); got != nRecords {
				t.Errorf("Unexpected amount of records; want %d, got %d", nRecords, got)
			}
		})
	}
}

func compareAction(t *testing.T, action config.Action, record actiondb.PipeLineRecord) {
	t.Helper()

	var recordConfig config.Action
	err := json.Unmarshal(record.Config, &recordConfig)
	if err != nil {
		t.Errorf("failed to unmarshal record config: %v, JSON: %s", err, string(record.Config))
		return
	}

	if !reflect.DeepEqual(action, recordConfig) {
		t.Errorf("record config does not match, want %v, got %v", action, recordConfig)
	}
}

func compareRecord(t *testing.T, want actiondb.PipeLineRecord, got actiondb.PipeLineRecord) {
	t.Helper()

	if want.PipeId != got.PipeId {
		t.Errorf("Bad pipeId: want %s, got %s,", want.PipeId, got.PipeId)
	}
	if want.Project != got.Project {
		t.Errorf("Bad project: want %s, got %s,", want.Project, got.Project)
	}
	if want.DeliveryId != got.DeliveryId {
		t.Errorf("Bad deliveryId, want %s, got %s", want.DeliveryId, got.DeliveryId)
	}
	if want.Output != got.Output {
		t.Errorf("Unexpected output in pipeline: want %v, got %v", want.Output, got.Output)
	}
	if want.Error != got.Error {
		t.Errorf("Unexpected error value in created record: want %v, got %v", want.Error, got.Error)
	}
	if got.CreatedAt == 0 {
		t.Errorf("Unexpected empty created date: want %d, got %d", want.CreatedAt, got.CreatedAt)
	}
	if want.EndedAt.Valid != got.EndedAt.Valid {
		t.Errorf("Unexpected emptiness of ended date: want %t, got %t", want.EndedAt.Valid, got.EndedAt.Valid)
	}
}
