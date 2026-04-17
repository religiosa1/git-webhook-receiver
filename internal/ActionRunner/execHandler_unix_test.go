//go:build unix

package ActionRunner

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"syscall"
	"testing"
)

// TestHelperProcessUnix is a subprocess helper for Unix-specific execHandler tests.
// Returns immediately when GO_TEST_HELPER_CMD is not "print_pgid".
func TestHelperProcessUnix(t *testing.T) {
	if os.Getenv("GO_TEST_HELPER_CMD") != "print_pgid" {
		return
	}
	fmt.Println(syscall.Getpgrp())
	os.Exit(0)
}

func TestExecHandlerSetpgid(t *testing.T) {
	sysProcAttr := &syscall.SysProcAttr{Setpgid: true}
	var stdout, stderr bytes.Buffer
	runner := newExecHandlerRunner(t, sysProcAttr, &stdout, &stderr)

	err := runScript(t, runner, context.Background(), helperCmd(t, "TestHelperProcessUnix", "print_pgid"))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	childPGID, err := strconv.Atoi(strings.TrimSpace(stdout.String()))
	if err != nil {
		t.Fatalf("failed to parse child PGID from output %q: %v", stdout.String(), err)
	}

	parentPGID := syscall.Getpgrp()
	if childPGID == parentPGID {
		t.Errorf("child PGID (%d) should differ from parent PGID (%d) when Setpgid=true", childPGID, parentPGID)
	}
}
