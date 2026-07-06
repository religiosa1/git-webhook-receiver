package actionrunner

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/religiosa1/git-webhook-receiver/internal/actionsdb"
	"github.com/religiosa1/git-webhook-receiver/internal/config"
	"github.com/religiosa1/git-webhook-receiver/internal/tmpoutput"
)

// failingTmpOutput is a tmpoutput.Manager whose Create always fails, used to
// simulate the "couldn't create the temporary output file" branch.
type failingTmpOutput struct{ err error }

func (f failingTmpOutput) Create(string) (io.Writer, error)                 { return nil, f.err }
func (f failingTmpOutput) Drain(string) (io.Reader, error)                  { return nil, f.err }
func (f failingTmpOutput) Close(string) error                               { return nil }
func (f failingTmpOutput) Reader(context.Context, string) (io.Reader, bool) { return nil, false }

func newTestActionsDB(t *testing.T) *actionsdb.ActionDB {
	t.Helper()
	db, err := actionsdb.New(filepath.Join(t.TempDir(), "actions.sqlite3"), 0)
	if err != nil {
		t.Fatalf("failed to create test actions db: %v", err)
	}
	t.Cleanup(func() { _ = db.Close() })
	return db
}

func makeExecArgs(pipeID string, cfg config.Action) ActionArgs {
	return ActionArgs{
		Logger:     slog.New(slog.NewTextHandler(io.Discard, nil)),
		DeliveryID: "delivery-1",
		Hash:       "abc123",
		Event:      "push",
		Branch:     "master",
		ActionDesc: ActionDescriptor{
			ActionIdentifier: ActionIdentifier{Index: 0, Project: "proj", PipeID: pipeID},
			GitProvider:      "github",
			Repo:             "user/repo",
			Config:           cfg,
		},
	}
}

// assertRecordClosedWithError checks the pipeline record exists, has been closed
// (EndedAt set) and carries a non-nil error whose message contains msgSubstr.
func assertRecordClosedWithError(t *testing.T, db *actionsdb.ActionDB, pipeID, msgSubstr string) {
	t.Helper()
	rec, err := db.GetPipelineRecord(pipeID)
	if err != nil {
		t.Fatalf("record %q was not persisted: %v", pipeID, err)
	}
	if rec.EndedAt == nil {
		t.Errorf("record %q was left open (EndedAt is nil)", pipeID)
	}
	if rec.Error == nil {
		t.Fatalf("record %q closed without an error, want one containing %q", pipeID, msgSubstr)
	}
	if !strings.Contains(rec.Error.Error(), msgSubstr) {
		t.Errorf("record %q error = %q; want it to contain %q", pipeID, rec.Error.Error(), msgSubstr)
	}
}

func assertOutputEmpty(t *testing.T, db *actionsdb.ActionDB, pipeID string) {
	t.Helper()
	out, err := db.GetPipelineOutput(pipeID)
	if err != nil {
		t.Fatalf("failed to read output for %q: %v", pipeID, err)
	}
	if len(out) != 0 {
		t.Errorf("output for %q = %q; want empty", pipeID, out)
	}
}

// pipelineError must wrap both the ErrPipeline sentinel and the underlying
// cause, so infra failures are distinguishable via errors.Is. This only holds
// on the live error chain; see ErrPipeline's doc for the DB-roundtrip caveat.
func TestPipelineErrorWrapsSentinelAndCause(t *testing.T) {
	cause := errors.New("disk on fire")
	err := pipelineError(cause)

	if !errors.Is(err, ErrPipeline) {
		t.Errorf("errors.Is(err, ErrPipeline) = false; want true (err = %q)", err)
	}
	if !errors.Is(err, cause) {
		t.Errorf("errors.Is(err, cause) = false; want true (err = %q)", err)
	}
}

// A successful run must close the pipeline record without an error and persist
// the captured output. The script uses the shell's `echo` builtin, so it runs
// in-process and stays portable.
func TestExecuteActionSuccessClosesRecordWithOutput(t *testing.T) {
	db := newTestActionsDB(t)
	r := &ActionRunner{actionsDB: db, tmpOutputMgr: tmpoutput.NewInMemoryTmpOutput(0)}

	pipeID := "pipe-success"
	args := makeExecArgs(pipeID, config.Action{
		Script:  "echo happy-path-output",
		Timeout: time.Minute,
	})

	r.executeAction(context.Background(), args)

	rec, err := db.GetPipelineRecord(pipeID)
	if err != nil {
		t.Fatalf("record %q was not persisted: %v", pipeID, err)
	}
	if rec.EndedAt == nil {
		t.Errorf("record %q was left open (EndedAt is nil)", pipeID)
	}
	if rec.Error != nil {
		t.Errorf("record %q closed with error %q; want none", pipeID, rec.Error)
	}

	out, err := db.GetPipelineOutput(pipeID)
	if err != nil {
		t.Fatalf("failed to read output for %q: %v", pipeID, err)
	}
	if !strings.Contains(string(out), "happy-path-output") {
		t.Errorf("output for %q = %q; want it to contain %q", pipeID, out, "happy-path-output")
	}
}

// A createEnv failure (here: forbidden command substitution) must still close
// the pipeline record, tagged with the environment-build error.
func TestExecuteActionEnvErrorClosesRecord(t *testing.T) {
	db := newTestActionsDB(t)
	r := &ActionRunner{actionsDB: db, tmpOutputMgr: tmpoutput.NewInMemoryTmpOutput(0)}

	pipeID := "pipe-env-error"
	args := makeExecArgs(pipeID, config.Action{
		Run:         []string{"true"},
		Environment: config.EnvList{"X=$(echo pwned)"},
	})

	r.executeAction(context.Background(), args)

	assertRecordClosedWithError(t, db, pipeID, "action environment")
	assertOutputEmpty(t, db, pipeID)
}

// A getSysProcAttr failure (here: an unknown run-as user) must still close the
// pipeline record, tagged with the process-attributes error.
func TestExecuteActionSysProcAttrErrorClosesRecord(t *testing.T) {
	db := newTestActionsDB(t)
	r := &ActionRunner{actionsDB: db, tmpOutputMgr: tmpoutput.NewInMemoryTmpOutput(0)}

	pipeID := "pipe-sysprocattr-error"
	// A non-empty user is rejected on non-unix, and fails user.Lookup on unix.
	args := makeExecArgs(pipeID, config.Action{
		Run:  []string{"true"},
		User: "nonexistent_user_for_test_zzz",
	})

	r.executeAction(context.Background(), args)

	assertRecordClosedWithError(t, db, pipeID, "process attributes")
	assertOutputEmpty(t, db, pipeID)
}

// Failing to create the temporary output file must still produce a closed
// pipeline record with empty output (the run never starts).
func TestExecuteActionTmpFileErrorCreatesEmptyRecord(t *testing.T) {
	db := newTestActionsDB(t)
	r := &ActionRunner{
		actionsDB:    db,
		tmpOutputMgr: failingTmpOutput{err: errors.New("disk on fire")},
	}

	pipeID := "pipe-tmpfile-error"
	args := makeExecArgs(pipeID, config.Action{Run: []string{"true"}})

	r.executeAction(context.Background(), args)

	assertRecordClosedWithError(t, db, pipeID, "temporary file")
	assertOutputEmpty(t, db, pipeID)
}

// With no actions DB configured, a tmp-file creation failure must simply bail
// out without panicking (nothing to record).
func TestExecuteActionTmpFileErrorWithoutDB(t *testing.T) {
	r := &ActionRunner{
		actionsDB:    nil,
		tmpOutputMgr: failingTmpOutput{err: errors.New("disk on fire")},
	}

	args := makeExecArgs("pipe-nodb", config.Action{Run: []string{"true"}})

	// Must not panic; there is no record to assert on.
	r.executeAction(context.Background(), args)
}
