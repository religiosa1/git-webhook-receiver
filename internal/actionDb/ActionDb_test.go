package actiondb_test

import (
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	actiondb "github.com/religiosa1/webhook-receiver/internal/actionDb"
	"github.com/religiosa1/webhook-receiver/internal/config"
)

func CompareAction(t *testing.T, action config.Action, record actiondb.PipeLineRecord) {
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

func TestActionDb(t *testing.T) {
	pipeId := "123"
	deliveryId := "321"
	action := config.Action{
		On:     "push",
		Branch: "main",
		Cwd:    "/var/www",
		User:   "www-data",
		Script: "whoami",
	}

	t.Run("susccesfully creates a db", func(t *testing.T) {
		_, err := actiondb.New(":memory:")
		if err != nil {
			t.Error((err))
		}
	})
	t.Run("creates a record", func(t *testing.T) {
		db, err := actiondb.New(":memory:")
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline rercord: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		if record.PipeId != pipeId {
			t.Errorf("Bad pipeId, got %s, want %s", record.PipeId, pipeId)
		}
		if record.DeliveryId != deliveryId {
			t.Errorf("Bad deliveryId, got %s, want %s", record.DeliveryId, deliveryId)
		}
		if record.Output.Valid {
			t.Errorf("Unexpted non-empty output in pipeline: %s", record.Output.String)
		}
		if record.Error.Valid {
			t.Errorf("Unexpted non-empty  error in created record: %s", record.Error.String)
		}
		if record.CreatedAt == 0 {
			t.Errorf("Unexpted empty created date: %d", record.CreatedAt)
		}
		if record.EndedAt.Valid {
			t.Errorf("Unexpted non-empty ended date: %d", record.EndedAt.Int64)
		}
		CompareAction(t, action, record)
	})

	// TODO
	t.Run("Successfull actions", func(t *testing.T) {
		t.Skip()
	})

	t.Run("updates a record on close with error", func(t *testing.T) {
		actionErr := errors.New("some error blah blah")
		actionOutput := "test output"
		db, err := actiondb.New(":memory:")
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Fatalf("Unable to create a pipeline rercord: %s", err)
		}

		err = db.CloseRecord(pipeId, actionErr, actionOutput)
		if err != nil {
			t.Fatalf("Unable to close a pipeline rercord: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		if record.PipeId != pipeId {
			t.Errorf("Bad pipeId, got %s, want %s", record.PipeId, pipeId)
		}
		if record.DeliveryId != deliveryId {
			t.Errorf("Bad deliveryId, got %s, want %s", record.DeliveryId, deliveryId)
		}
		if !record.Output.Valid || record.Output.String != actionOutput {
			t.Errorf("Bad output value, want %s, got %v", actionOutput, record.Output)
		}
		if !record.Error.Valid || record.Error.String != actionErr.Error() {
			t.Errorf("Bad record's error value, want %s, got %v", actionErr, record.Error)
		}
		if record.CreatedAt == 0 {
			t.Errorf("Unexpted empty created date: %d", record.CreatedAt)
		}
		if !record.EndedAt.Valid {
			t.Errorf("Unexpted empty ended date: %v", record.EndedAt)
		}
		CompareAction(t, action, record)
	})

	t.Run("db keeps data persistently", func(t *testing.T) {
		tmpdir := t.TempDir()
		tmpfile, err := os.CreateTemp(tmpdir, "*.sqlite3")
		if err != nil {
			t.Fatalf("Unable to create a tempfile for db: %s", err)
		}
		defer tmpfile.Close()
		db, err := actiondb.New(tmpfile.Name())
		if err != nil {
			t.Fatalf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Fatalf("Unable to create a record: %s", err)
		}

		err = db.Close()
		if err != nil {
			t.Fatalf("Unable to close the db: %s", err)
		}

		db2, err := actiondb.New(tmpfile.Name())
		if err != nil {
			t.Fatalf("Unable to open the db for the second time: %s", err)
		}

		record, err := db2.GetPipelineRecord(pipeId)
		if err != nil {
			t.Fatalf("Unable to retrieve the created record: %s", err)
		}

		if record.PipeId != pipeId {
			t.Errorf("Bad pipeId, got %s, want %s", record.PipeId, pipeId)
		}
		if record.DeliveryId != deliveryId {
			t.Errorf("Bad deliveryId, got %s, want %s", record.DeliveryId, deliveryId)
		}
		if record.Output.Valid {
			t.Errorf("Unexpted non-empty output in pipeline: %s", record.Output.String)
		}
		if record.Error.Valid {
			t.Errorf("Unexpted non-empty  error in created record: %s", record.Error.String)
		}
		if record.CreatedAt == 0 {
			t.Errorf("Unexpted empty created date: %d", record.CreatedAt)
		}
		if record.EndedAt.Valid {
			t.Errorf("Unexpted non-empty ended date: %d", record.EndedAt.Int64)
		}
		CompareAction(t, action, record)
	})
}
