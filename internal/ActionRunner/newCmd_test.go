package ActionRunner

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"os/signal"
	"runtime"
	"testing"
	"time"
)

// TestSubprocessGraceful is a subprocess helper used by newCmd graceful-shutdown tests.
// It returns immediately when GO_TEST_HELPER_GRACEFUL is not set.
func TestSubprocessGraceful(t *testing.T) {
	switch os.Getenv("GO_TEST_HELPER_GRACEFUL") {
	case "":
		return
	case "sigint-exit2":
		// Catches SIGINT and exits with code 2 to confirm interrupt was delivered.
		sig := make(chan os.Signal, 1)
		signal.Notify(sig, os.Interrupt)
		<-sig
		os.Exit(2)
	case "sigint-ignore":
		// Ignores SIGINT — requires a force kill via WaitDelay.
		signal.Ignore(os.Interrupt)
		time.Sleep(time.Minute)
		os.Exit(0)
	default:
		os.Exit(1)
	}
}

func gracefulSubprocess(t *testing.T, cmd string) (path string, args []string) {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatalf("os.Executable: %v", err)
	}
	t.Setenv("GO_TEST_HELPER_GRACEFUL", cmd)
	return exe, []string{"-test.run=^TestSubprocessGraceful$", "-test.v=false"}
}

func TestNewCmdWaitDelay(t *testing.T) {
	want := 250 * time.Millisecond
	cmd := newCmd(context.Background(), "true", nil, nil, want)
	if cmd.WaitDelay != want {
		t.Errorf("WaitDelay: want %v, got %v", want, cmd.WaitDelay)
	}
}

// TestNewCmdGracefulCancelSendsInterrupt verifies that cancelling the context
// delivers SIGINT to the process rather than killing it outright.
func TestNewCmdGracefulCancelSendsInterrupt(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT semantics differ on Windows")
	}

	path, args := gracefulSubprocess(t, "sigint-exit2")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := newCmd(ctx, path, args, nil, 2*time.Second)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	// Allow subprocess to reach signal.Notify before we cancel.
	time.Sleep(200 * time.Millisecond)
	cancel()

	waitErr := cmd.Wait()

	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got: %v (%T)", waitErr, waitErr)
	}
	if got := exitErr.ExitCode(); got != 2 {
		t.Errorf("want exit code 2 (SIGINT caught by subprocess), got %d", got)
	}
}

// TestNewCmdForceKillAfterGracefulTimeout verifies that a process which ignores
// SIGINT is forcibly killed once WaitDelay elapses.
func TestNewCmdForceKillAfterGracefulTimeout(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("SIGINT semantics differ on Windows")
	}

	path, args := gracefulSubprocess(t, "sigint-ignore")

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	cmd := newCmd(ctx, path, args, nil, 50*time.Millisecond)
	if err := cmd.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}
	time.Sleep(200 * time.Millisecond)
	cancel()

	waitErr := cmd.Wait()

	var exitErr *exec.ExitError
	if !errors.As(waitErr, &exitErr) {
		t.Fatalf("expected *exec.ExitError, got: %v (%T)", waitErr, waitErr)
	}
	if got := exitErr.ExitCode(); got != -1 {
		t.Errorf("want exit code -1 (killed by signal), got %d", got)
	}
}
