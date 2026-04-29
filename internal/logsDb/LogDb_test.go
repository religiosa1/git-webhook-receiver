package logsDb_test

import (
	"database/sql"
	"log/slog"
	"os"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/logsDb"
)

var testEntry = logsDb.LogEntry{
	Level:      int(slog.LevelInfo),
	Project:    sql.NullString{Valid: true, String: "testProject"},
	DeliveryID: sql.NullString{Valid: true, String: "testDeliveryId"},
	PipeID:     sql.NullString{Valid: true, String: "testPipeId"},
	Message:    "testMessage",
	Data:       `{"test":123}`,
}

func TestLogDb(t *testing.T) {
	t.Run("Creates a db", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}
		err = db.Close()
		if err != nil {
			t.Fatalf("Error while closing the db: %s", err)
		}
	})

	t.Run("Returns nil on empty dbfilename", func(t *testing.T) {
		db, err := logsDb.New("")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}
		if db != nil {
			t.Fatalf("Expecting db to be nil, but got this instead: %v", db)
		}
	})

	t.Run("Creates a record in the db", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}

		err = db.CreateEntry(testEntry)
		if err != nil {
			t.Fatalf("Error while creating an entry: %s", err)
		}
		entries, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(entries.Items); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}

		CompareEntries(t, testEntry, entries.Items[0])
	})

	t.Run("Db can be opened repeatedly", func(t *testing.T) {
		tmpFile, err := os.CreateTemp(t.TempDir(), "*.sqlite3")
		if err != nil {
			t.Fatalf("Unable to create a tempfile for db: %s", err)
		}
		defer func() {
			_ = tmpFile.Close()
		}()
		db, err := logsDb.New(tmpFile.Name())
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}
		err = db.CreateEntry(testEntry)
		if err != nil {
			t.Fatalf("Error while creating an entry: %s", err)
		}
		_ = db.Close()
		db2, err := logsDb.New(tmpFile.Name())
		if err != nil {
			t.Fatalf("Error while opening the db for the second time: %s", err)
		}
		entries, err := db2.GetEntryFiltered(logsDb.GetEntryFilteredQuery{})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(entries.Items); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}

		CompareEntries(t, testEntry, entries.Items[0])
	})

	t.Run("Allows to retrieve entries with pagination", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}

		testEntry2 := testEntry
		testEntry2.Message = "message2"

		testEntry3 := testEntry
		testEntry3.Message = "message3"

		for _, entry := range [...]logsDb.LogEntry{testEntry, testEntry2, testEntry3} {
			err = db.CreateEntry(entry)
			if err != nil {
				t.Fatalf("Error while creating an entry: %s", err)
			}
		}

		page1, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{PageSize: 2})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(page1.Items); l != 2 {
			t.Errorf("Unexpected number of entries in the db, want 2, got %d", l)
		}
		// Should be in descending order
		CompareEntries(t, testEntry3, page1.Items[0])
		CompareEntries(t, testEntry2, page1.Items[1])

		if page1.Cursor == nil {
			t.Fatalf("Expected to received next page cursor, received nil")
		}

		page2, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{Cursor: *page1.Cursor})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(page2.Items); l != 1 {
			t.Errorf("Unexpected number of entries in the db, want 1, got %d", l)
		}
		CompareEntries(t, testEntry, page2.Items[0])
	})
}

func TestLogDbFiltering(t *testing.T) {
	t.Run("Single item-filtering", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}

		projectEntry := testEntry
		projectEntry.Project = sql.NullString{Valid: true, String: "project-search"}

		deliveryEntry := testEntry
		deliveryEntry.DeliveryID = sql.NullString{Valid: true, String: "delivery-search"}

		pipeEntry := testEntry
		pipeEntry.PipeID = sql.NullString{Valid: true, String: "pipe-search"}

		messageEntry := testEntry
		messageEntry.Message = "message-search"

		allEntries := []logsDb.LogEntry{
			testEntry,
			projectEntry,
			deliveryEntry,
			pipeEntry,
			messageEntry,
		}
		for _, entry := range allEntries {
			err := db.CreateEntry(entry)
			if err != nil {
				t.Fatalf("error while creating an entry %v: %s", entry, err)
			}
		}

		s1, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{Project: "project-search"})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s1.Items); l != 1 {
			t.Errorf("Unexpected number of entries returned, want 1, got %d", l)
		}
		CompareEntries(t, projectEntry, s1.Items[0])

		s2, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{DeliveryID: "delivery-search"})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s2.Items); l != 1 {
			t.Errorf("Unexpected number of entries returned, want 1, got %d", l)
		}
		CompareEntries(t, deliveryEntry, s2.Items[0])

		s3, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{PipeID: "pipe-search"})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s3.Items); l != 1 {
			t.Errorf("Unexpected number of entries returned, want 1, got %d", l)
		}
		CompareEntries(t, pipeEntry, s3.Items[0])

		s4, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{Message: "message-search"})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s4.Items); l != 1 {
			t.Errorf("Unexpected number of entries returned, want 1, got %d", l)
		}
		CompareEntries(t, messageEntry, s4.Items[0])
	})

	t.Run("All of filtering conditions must match", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}

		pipeID := "test-pipeid-search"
		delivery := "test-delivery-search"

		entryA := testEntry
		entryA.PipeID = sql.NullString{Valid: true, String: pipeID}

		entryB := testEntry
		entryB.DeliveryID = sql.NullString{Valid: true, String: delivery}

		entryAB := testEntry
		entryAB.PipeID = sql.NullString{Valid: true, String: pipeID}
		entryAB.DeliveryID = sql.NullString{Valid: true, String: delivery}

		allEntries := []logsDb.LogEntry{
			testEntry,
			entryA,
			entryB,
			entryAB,
		}
		for _, entry := range allEntries {
			err := db.CreateEntry(entry)
			if err != nil {
				t.Fatalf("error while creating an entry %v: %s", entry, err)
			}
		}

		s1, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{DeliveryID: delivery, PipeID: pipeID})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s1.Items); l != 1 {
			t.Errorf("Unexpected number of entries returned, want 1, got %d", l)
		}
		CompareEntries(t, entryAB, s1.Items[0])
	})

	t.Run("Any of the log levels can match", func(t *testing.T) {
		db, err := logsDb.New(":memory:")
		if err != nil {
			t.Fatalf("Error while opening a db: %s", err)
		}

		entryA := testEntry
		entryA.Level = int(slog.LevelError)

		entryB := testEntry
		entryB.Level = int(slog.LevelWarn)

		allEntries := []logsDb.LogEntry{
			testEntry,
			entryA,
			entryB,
		}
		for _, entry := range allEntries {
			err := db.CreateEntry(entry)
			if err != nil {
				t.Fatalf("error while creating an entry %v: %s", entry, err)
			}
		}

		s1, err := db.GetEntryFiltered(logsDb.GetEntryFilteredQuery{Levels: []int{
			int(slog.LevelError),
			int(slog.LevelWarn),
		}})
		if err != nil {
			t.Fatalf("Error while retrieving entries: %s", err)
		}
		if l := len(s1.Items); l != 2 {
			t.Errorf("Unexpected number of entries returned, want 2, got %d", l)
		}
		// Must be in reversed order
		CompareEntries(t, entryB, s1.Items[0])
		CompareEntries(t, entryA, s1.Items[1])
	})
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
	if want.DeliveryID != got.DeliveryID {
		t.Errorf("Wrong DeliveryId value, want %v got %v", want.DeliveryID, got.DeliveryID)
		return false
	}
	if want.PipeID != got.PipeID {
		t.Errorf("Wrong PipeId value, want %v got %v", want.PipeID, got.PipeID)
		return false
	}
	if want.Message != got.Message {
		t.Errorf("Wrong Message value, want %s got %s", want.Message, got.Message)
		return false
	}
	if want, got := want.Data, got.Data; want != got {
		t.Errorf("Wrong Data value, want %s got %s", want, got)
		return false
	}
	return true
}
