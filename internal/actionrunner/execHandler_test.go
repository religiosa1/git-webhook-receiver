package actionrunner

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"syscall"
	"testing"
	"time"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/interp"
	"mvdan.cc/sh/v3/syntax"
)

// TestHelperProcess is a subprocess helper invoked by execHandler tests.
// Returns immediately when GO_TEST_HELPER_CMD is not set.
func TestHelperProcess(t *testing.T) {
	switch os.Getenv("GO_TEST_HELPER_CMD") {
	case "":
		return
	case "exit0":
		os.Exit(0)
	case "exit42":
		os.Exit(42)
	case "sleep":
		time.Sleep(time.Minute)
		os.Exit(0)
	default:
		fmt.Fprintf(os.Stderr, "unknown helper command: %s\n", os.Getenv("GO_TEST_HELPER_CMD"))
		os.Exit(1)
	}
}

func newExecHandlerRunner(t *testing.T, sysProcAttr *syscall.SysProcAttr, env []string, stdout, stderr io.Writer) *interp.Runner {
	t.Helper()
	runner, err := interp.New(
		interp.Env(expand.ListEnviron(env...)),
		interp.ExecHandlers(execHandler(sysProcAttr, 0)),
		interp.StdIO(nil, stdout, stderr),
	)
	if err != nil {
		t.Fatalf("interp.New: %v", err)
	}
	return runner
}

func runScript(t *testing.T, runner *interp.Runner, ctx context.Context, script string) error {
	t.Helper()
	parsed, err := syntax.NewParser().Parse(strings.NewReader(script), "")
	if err != nil {
		t.Fatalf("parse script: %v", err)
	}
	return runner.Run(ctx, parsed)
}

// helperCmd returns a shell command invoking the test binary as a subprocess
// helper, plus the env that must be supplied to the runner so the subprocess
// sees GO_TEST_HELPER_CMD. execHandler isolates the child environment, so the
// var must be passed explicitly rather than inherited from the parent process.
func helperCmd(t *testing.T, helperFunc, cmd string) (script string, env []string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	script = fmt.Sprintf("%s -test.run=%s -test.v=false", exe, helperFunc)
	env = []string{"GO_TEST_HELPER_CMD=" + cmd}
	return script, env
}

func TestExecHandlerExitZero(t *testing.T) {
	var out bytes.Buffer
	script, env := helperCmd(t, "TestHelperProcess", "exit0")
	runner := newExecHandlerRunner(t, nil, env, &out, &out)

	err := runScript(t, runner, context.Background(), script)
	if err != nil {
		t.Fatalf("expected nil, got: %v", err)
	}
}

func TestExecHandlerNonZeroExit(t *testing.T) {
	var out bytes.Buffer
	script, env := helperCmd(t, "TestHelperProcess", "exit42")
	runner := newExecHandlerRunner(t, nil, env, &out, &out)

	err := runScript(t, runner, context.Background(), script)

	var status interp.ExitStatus
	if !errors.As(err, &status) {
		t.Fatalf("expected ExitStatus error, got: %v", err)
	}
	if uint8(status) != 42 {
		t.Errorf("expected exit status 42, got %d", uint8(status))
	}
}

func TestExecHandlerCommandNotFound(t *testing.T) {
	var out bytes.Buffer
	runner := newExecHandlerRunner(t, nil, nil, &out, &out)

	err := runScript(t, runner, context.Background(), "/nonexistent-command-xyz123")

	var status interp.ExitStatus
	if !errors.As(err, &status) {
		t.Fatalf("expected ExitStatus error, got: %v", err)
	}
	if uint8(status) != 127 {
		t.Errorf("expected exit status 127, got %d", uint8(status))
	}
}

func TestExecHandlerContextCancellation(t *testing.T) {
	var out bytes.Buffer
	script, env := helperCmd(t, "TestHelperProcess", "sleep")
	runner := newExecHandlerRunner(t, nil, env, &out, &out)

	ctx, cancel := context.WithTimeout(context.Background(), 500*time.Millisecond)
	defer cancel()

	err := runScript(t, runner, ctx, script)

	if !errors.Is(err, context.DeadlineExceeded) {
		t.Errorf("expected context.DeadlineExceeded, got: %v", err)
	}
}
