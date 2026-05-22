package api_test

import (
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

func newTestActionDB(t *testing.T) *actionsdb.ActionDB {
	t.Helper()
	db, err := actionsdb.New(":memory:", 1000)
	if err != nil {
		t.Fatalf("failed to create test DB: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

var testAction = config.Action{
	On:     "push",
	Branch: "main",
	Script: "echo test",
}

func seedActionDBRecord(t *testing.T, db *actionsdb.ActionDB, pipeID, project, deliveryID string) {
	t.Helper()
	if err := db.CreateRecord(pipeID, project, deliveryID, testAction); err != nil {
		t.Fatalf("seed record %s: %v", pipeID, err)
	}
}

func seedActionDBCompletedRecord(t *testing.T, db *actionsdb.ActionDB, pipeID, project, deliveryID, output string, cmdErr error) {
	t.Helper()
	seedActionDBRecord(t, db, pipeID, project, deliveryID)
	if err := db.CloseRecord(pipeID, cmdErr, []byte(output)); err != nil {
		t.Fatalf("close record %s: %v", pipeID, err)
	}
}
