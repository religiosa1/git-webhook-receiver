package actiondb_test

import (
	"database/sql"
	"encoding/json"
	"errors"
	"os"
	"reflect"
	"testing"

	actiondb "github.com/religiosa1/webhook-receiver/internal/actionDb"
	"github.com/religiosa1/webhook-receiver/internal/config"
)

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
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline rercord: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			DeliveryId: deliveryId,
		}

		CompareRecord(t, want, record)
		CompareAction(t, action, record)
	})

	t.Run("Close successful action", func(t *testing.T) {
		actionOutput := "test output"
		db, err := actiondb.New(":memory:")
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline rercord: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, actionOutput)
		if err != nil {
			t.Errorf("Unable to close a pipeline rercord: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			DeliveryId: deliveryId,
			Output:     sql.NullString{Valid: true, String: actionOutput},
			Error:      sql.NullString{Valid: false},
			EndedAt:    sql.NullInt64{Valid: true},
		}
		CompareRecord(t, want, record)
		CompareAction(t, action, record)
	})

	t.Run("Close errored action", func(t *testing.T) {
		actionErr := errors.New("some error blah blah")
		actionOutput := "test output"
		db, err := actiondb.New(":memory:")
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline rercord: %s", err)
		}

		err = db.CloseRecord(pipeId, actionErr, actionOutput)
		if err != nil {
			t.Errorf("Unable to close a pipeline rercord: %s", err)
		}

		record, err := db.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			DeliveryId: deliveryId,
			Output:     sql.NullString{Valid: true, String: actionOutput},
			Error:      sql.NullString{Valid: true, String: actionErr.Error()},
			EndedAt:    sql.NullInt64{Valid: true},
		}
		CompareRecord(t, want, record)
		CompareAction(t, action, record)
	})

	t.Run("An action can only be closed once", func(t *testing.T) {
		db, err := actiondb.New(":memory:")
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a pipeline rercord: %s", err)
		}

		err = db.CloseRecord(pipeId, nil, "")
		if err != nil {
			t.Errorf("Unable to close a pipeline rercord: %s", err)
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
		defer tmpfile.Close()
		db, err := actiondb.New(tmpfile.Name())
		if err != nil {
			t.Errorf("Unable to create a db: %s", err)
		}

		err = db.CreateRecord(pipeId, deliveryId, action)
		if err != nil {
			t.Errorf("Unable to create a record: %s", err)
		}

		err = db.Close()
		if err != nil {
			t.Errorf("Unable to close the db: %s", err)
		}

		db2, err := actiondb.New(tmpfile.Name())
		if err != nil {
			t.Errorf("Unable to open the db for the second time: %s", err)
		}

		record, err := db2.GetPipelineRecord(pipeId)
		if err != nil {
			t.Errorf("Unable to retrieve the created record: %s", err)
		}

		want := actiondb.PipeLineRecord{
			PipeId:     pipeId,
			DeliveryId: deliveryId,
		}

		CompareRecord(t, want, record)
		CompareAction(t, action, record)
	})
}

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

func CompareRecord(t *testing.T, want actiondb.PipeLineRecord, got actiondb.PipeLineRecord) {
	t.Helper()

	if want.PipeId != got.PipeId {
		t.Errorf("Bad pipeId: want %s, got %s,", want.PipeId, got.PipeId)
	}
	if want.DeliveryId != got.DeliveryId {
		t.Errorf("Bad deliveryId, want %s, got %s", want.DeliveryId, got.DeliveryId)
	}
	if want.Output != got.Output {
		t.Errorf("Unexpted output in pipeline: want %v, got %v", want.Output, got.Output)
	}
	if want.Error != got.Error {
		t.Errorf("Unexpted error value in created record: want %v, got %v", want.Error, got.Error)
	}
	if got.CreatedAt == 0 {
		t.Errorf("Unexpted empty created date: want %d, got %d", want.CreatedAt, got.CreatedAt)
	}
	if want.EndedAt.Valid != got.EndedAt.Valid {
		t.Errorf("Unexpted emptiness of ended date: want %t, got %t", want.EndedAt.Valid, got.EndedAt.Valid)
	}
}
