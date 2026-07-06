package actionrunner

import (
	"bytes"
	"context"
	"strings"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

func TestExecuteActionScriptErrExitAborts(t *testing.T) {
	var out bytes.Buffer
	action := config.Action{Script: "false\necho after-failure"}

	err := executeActionScript(context.Background(), action, nil, nil, &out)

	if err == nil {
		t.Error("expected a non-nil error from the failed command, got nil")
	}
	if strings.Contains(out.String(), "after-failure") {
		t.Errorf("command after the failure ran; output = %q, want it aborted", out.String())
	}
}

func TestExecuteActionScriptSetPlusEOverrides(t *testing.T) {
	var out bytes.Buffer
	action := config.Action{Script: "set +e\nfalse\necho after-failure"}

	err := executeActionScript(context.Background(), action, nil, nil, &out)
	if err != nil {
		t.Errorf("expected nil error after `set +e`, got: %v", err)
	}
	if !strings.Contains(out.String(), "after-failure") {
		t.Errorf("command after the failure did not run; output = %q", out.String())
	}
}
