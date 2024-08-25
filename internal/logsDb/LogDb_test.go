package logsDb_test

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/religiosa1/webhook-receiver/internal/logsDb"
)

func TestLogDb(t *testing.T) {
	testEntry := logsDb.LogEntry{
		Level:      int(slog.LevelInfo),
		Project:    sql.NullString{Valid: true, String: "testProject"},
		DeliveryId: sql.NullString{Valid: true, String: "testDeliveryId"},
		PipeId:     sql.NullString{Valid: true, String: "testPipeId"},
		Message:    "testMessage",
		Data:       "testData",
	}

	t.Run("Creates a db", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Errorf("Error while opening a db: %s", err)
		}
		err = db.Close()
		if err != nil {
			t.Errorf("Error while closing the db: %s", err)
		}
	})

	t.Run("Returns nil on empty dbfilename", func(t *testing.T) {
		db, err := logsDb.New("")
		if err != nil {
			t.Errorf("Error while opening a db: %s", err)
		}
		if db != nil {
			t.Errorf("Expeting db to be nill, but got this instead: %v", db)
		}
	})

	t.Run("Creates a record in the db", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Errorf("Error while opening a db: %s", err)
		}

		err = db.CreateEntry(testEntry)
		if err != nil {
			t.Errorf("Error while creating an entry: %s", err)
		}
		entries, err := db.GetEntry(logsDb.GetEntryQuery{})
		if err != nil {
			t.Errorf("Error while retrieving entries: %s", err)
		}
		if l := len(entries); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}

		CompareEntries(t, testEntry, entries[0])
	})

	t.Run("Db can be opened repeatedly", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "*.sqlite3")
		if err != nil {
			t.Errorf("Unable to create a tempfile for db: %s", err)
		}
		defer tmpFile.Close()
		db, err := logsDb.New(tmpFile.Name())
		if err != nil {
			t.Errorf("Error while opening a db: %s", err)
		}
		err = db.CreateEntry(testEntry)
		if err != nil {
			t.Errorf("Error while creating an entry: %s", err)
		}
		db.Close()
		db2, err := logsDb.New(tmpFile.Name())
		if err != nil {
			t.Errorf("Error while opening the db for the second time: %s", err)
		}
		entries, err := db2.GetEntry(logsDb.GetEntryQuery{})
		if err != nil {
			t.Errorf("Error while retrieving entries: %s", err)
		}
		if l := len(entries); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}

		CompareEntries(t, testEntry, entries[0])
	})

	t.Run("Allows to retrieve entries with pagination", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Errorf("Error while opening a db: %s", err)
		}

		testEntry2 := testEntry
		testEntry2.Message = "message2"

		testEntry3 := testEntry
		testEntry3.Message = "message3"

		err = db.CreateEntry(testEntry)
		if err != nil {
			t.Errorf("Error while creating an entry: %s", err)
		}
		err = db.CreateEntry(testEntry2)
		if err != nil {
			t.Errorf("Error while creating an entry: %s", err)
		}
		err = db.CreateEntry(testEntry3)
		if err != nil {
			t.Errorf("Error while creating an entry: %s", err)
		}

		page1, err := db.GetEntry(logsDb.GetEntryQuery{PageSize: 2})
		if err != nil {
			t.Errorf("Error while retrieving entries: %s", err)
		}
		if l := len(page1); l != 2 {
			t.Errorf("Unexpected number of entries in the db, want 2, got %d", l)
		}
		CompareEntries(t, testEntry, page1[0])
		CompareEntries(t, testEntry2, page1[1])

		page2, err := db.GetEntry(logsDb.GetEntryQuery{CursorId: page1[1].Id, CursorTs: page1[1].Ts})
		if err != nil {
			t.Errorf("Error while retrieving entries: %s", err)
		}
		if l := len(page2); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}
		CompareEntries(t, testEntry3, page2[0])
	})

	// TODO filtering queries tests
}

func CompareEntries(t *testing.T, want logsDb.LogEntry, got logsDb.LogEntry) bool {
	t.Helper()
	if want.Level != got.Level {
		t.Errorf("Wrong level value, want %d got %d", want.Level, got.Level)
		return false
	}
	if want.Project != got.Project {
		t.Errorf("Wrong Project, want %v got %v", want.Project, got.Project)
		return false
	}
	if want.DeliveryId != got.DeliveryId {
		t.Errorf("Wrong DeliveryId value, want %v got %v", want.DeliveryId, got.DeliveryId)
		return false
	}
	if want.PipeId != got.PipeId {
		t.Errorf("Wrong PipeId value, want %v got %v", want.PipeId, got.PipeId)
		return false
	}
	if want.Message != got.Message {
		t.Errorf("Wrong Message value, want %s got %s", want.Message, got.Message)
		return false
	}
	if want.Data != got.Data {
		t.Errorf("Wrong Data value, want %s got %s", want.Data, got.Data)
		return false
	}
	return true
}
