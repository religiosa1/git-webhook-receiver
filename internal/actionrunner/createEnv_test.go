package actionrunner

import (
	"slices"
	"strings"
	"testing"

	"github.com/religiosa1/git-webhook-receiver/internal/config"
)

func makeArgs(environment []string) ActionArgs {
	return ActionArgs{
		DeliveryID: "delivery-1",
		Hash:       "abc123",
		Event:      "push",
		Branch:     "master",
		ActionDesc: ActionDescriptor{
			ActionIdentifier: ActionIdentifier{
				Index:   0,
				Project: "proj",
				PipeID:  "pipe-1",
			},
			GitProvider: "github",
			Repo:        "user/repo",
			Config:      config.Action{Environment: environment},
		},
	}
}

// envValue returns the last value for key (matching os/exec's last-wins rule).
func envValue(env []string, key string) (string, bool) {
	val, ok := "", false
	for _, e := range env {
		if k, v, found := strings.Cut(e, "="); found && k == key {
			val, ok = v, true
		}
	}
	return val, ok
}

func TestCreateEnvBuiltins(t *testing.T) {
	env, err := createEnv(makeArgs(nil), "")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}
	want := map[string]string{
		"PROJECT_NAME": "proj",
		"GIT_COMMIT":   "abc123",
		"GIT_PROVIDER": "github",
		"GIT_REPO":     "user/repo",
		"GIT_BRANCH":   "master",
		"GIT_EVENT":    "push",
		"DELIVERY_ID":  "delivery-1",
	}
	for k, v := range want {
		if got, ok := envValue(env, k); !ok || got != v {
			t.Errorf("env[%q] = %q, %v; want %q", k, got, ok, v)
		}
	}
}

func TestCreateEnvCwdAndTmpDir(t *testing.T) {
	args := makeArgs([]string{"CLONE_TARGET=${TMPDIR}", "DEST=${CWD}"})
	args.ActionDesc.Config.Cwd = "/var/www/app"

	env, err := createEnv(args, "/tmp/git-webhook-receiver-xyz")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}
	cases := map[string]string{
		"CWD":          "/var/www/app",
		"TMPDIR":       "/tmp/git-webhook-receiver-xyz",
		"CLONE_TARGET": "/tmp/git-webhook-receiver-xyz", // user entries can reference $TMPDIR
		"DEST":         "/var/www/app",                  // ...and $CWD
	}
	for k, v := range cases {
		if got, ok := envValue(env, k); !ok || got != v {
			t.Errorf("env[%q] = %q, %v; want %q", k, got, ok, v)
		}
	}

	// No temp dir requested -> TMPDIR must be absent, not empty.
	env, err = createEnv(makeArgs(nil), "")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}
	if _, ok := envValue(env, "TMPDIR"); ok {
		t.Error("TMPDIR present when no temp dir was requested")
	}
}

func TestCreateEnvInterpolation(t *testing.T) {
	t.Setenv("MY_TOKEN", "s3cr3t")

	env, err := createEnv(makeArgs([]string{
		"TOKEN=${MY_TOKEN}",
		"WITH_DEFAULT=${MISSING:-fallback}",
		"REPLACE=${MY_TOKEN:+present}",
		"REF_BUILTIN=${GIT_COMMIT}",
	}), "")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}

	cases := map[string]string{
		"TOKEN":        "s3cr3t",
		"WITH_DEFAULT": "fallback",
		"REPLACE":      "present",
		"REF_BUILTIN":  "abc123", // interpolation sees the built-in short list too
	}
	for k, v := range cases {
		if got, _ := envValue(env, k); got != v {
			t.Errorf("env[%q] = %q; want %q", k, got, v)
		}
	}
}

func TestCreateEnvOverridesBuiltin(t *testing.T) {
	env, err := createEnv(makeArgs([]string{"GIT_COMMIT=overridden"}), "")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}
	// os/exec uses the last value for a duplicate key -> user entry wins.
	if got, _ := envValue(env, "GIT_COMMIT"); got != "overridden" {
		t.Errorf("GIT_COMMIT = %q; want overridden", got)
	}
	// The override must be positioned after the built-in in the slice.
	builtinIdx := slices.Index(env, "GIT_COMMIT=abc123")
	overrideIdx := slices.Index(env, "GIT_COMMIT=overridden")
	if builtinIdx == -1 || overrideIdx == -1 || overrideIdx < builtinIdx {
		t.Errorf("override not appended after builtin: builtin=%d override=%d", builtinIdx, overrideIdx)
	}
}

func TestCreateEnvCascadingReferences(t *testing.T) {
	// Simulates the flattened root -> project -> action list the config parser
	// produces: later entries reference earlier ones, and a repeated key wins.
	env, err := createEnv(makeArgs([]string{
		"ROOT=base",           // root level
		"CHILD=${ROOT}/child", // project references root
		"LEAF=${CHILD}/leaf",  // action references project
		"ROOT=overridden",     // action overrides root
	}), "")
	if err != nil {
		t.Fatalf("createEnv returned error: %v", err)
	}
	cases := map[string]string{
		"CHILD": "base/child",
		"LEAF":  "base/child/leaf",
		"ROOT":  "overridden", // last-wins
	}
	for k, v := range cases {
		if got, _ := envValue(env, k); got != v {
			t.Errorf("env[%q] = %q; want %q", k, got, v)
		}
	}
}

func TestCreateEnvUnsetRequiredErrors(t *testing.T) {
	_, err := createEnv(makeArgs([]string{"X=${DEFINITELY_MISSING_VAR:?is required}"}), "")
	if err == nil {
		t.Fatal("expected error for unset required variable, got nil")
	}
	if !strings.Contains(err.Error(), "is required") {
		t.Errorf("error %q does not contain the custom message", err)
	}
}

func TestCreateEnvNoCommandSubstitution(t *testing.T) {
	_, err := createEnv(makeArgs([]string{"X=$(echo pwned)"}), "")
	if err == nil {
		t.Fatal("expected error for command substitution, got nil")
	}
}
