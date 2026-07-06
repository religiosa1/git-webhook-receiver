package actionrunner

import (
	"fmt"
	"os"
	"strings"

	"mvdan.cc/sh/v3/expand"
	"mvdan.cc/sh/v3/syntax"
)

// passthroughEnv lists parent-process variables forwarded into the otherwise
// isolated action environment. PATH lets actions resolve binaries by name;
// HOME/USER let tools like git find their config and identity.
var passthroughEnv = []string{"PATH", "HOME", "USER"}

// createEnv creates an environment for the action runner, where each entry is in
// the form of "key=value".
//
// tmpDir, when non-empty, is the runner-managed temporary directory exposed to
// the action as $TMPDIR (see WithTempDir); empty means the action didn't request
// one. CWD mirrors the action's `cwd` config, keeping a single source of truth.
//
// The action's config `environment` entries are interpolated and appended last,
// so they may override any built-in or passed-through variable (for a duplicate
// key os/exec uses the last value in the slice).
func createEnv(args ActionArgs, tmpDir string) ([]string, error) {
	env := []string{
		fmt.Sprintf("PROJECT_NAME=%s", args.ActionDesc.Project),
		fmt.Sprintf("ACTION_IDX=%d", args.ActionDesc.Index),
		fmt.Sprintf("PIPELINE_ID=%s", args.ActionDesc.PipeID),
		fmt.Sprintf("GIT_COMMIT=%s", args.Hash),
		fmt.Sprintf("DELIVERY_ID=%s", args.DeliveryID),
		// action desc
		fmt.Sprintf("GIT_PROVIDER=%s", args.ActionDesc.GitProvider),
		fmt.Sprintf("GIT_REPO=%s", args.ActionDesc.Repo),
		fmt.Sprintf("GIT_BRANCH=%s", args.Branch),
		fmt.Sprintf("GIT_EVENT=%s", args.Event),
		fmt.Sprintf("CWD=%s", args.ActionDesc.Config.Cwd),
	}
	if tmpDir != "" {
		env = append(env, fmt.Sprintf("TMPDIR=%s", tmpDir))
	}
	for _, key := range passthroughEnv {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}

	userEnv, err := expandEnvEntries(args.ActionDesc.Config.Environment, env)
	if err != nil {
		return nil, err
	}
	return append(env, userEnv...), nil
}

// expandEnvEntries interpolates the "KEY=VALUE" config entries, resolving
// ${VAR}, ${VAR:-default}, ${VAR:?error} and the like against the receiver's
// process environment layered under `base` (so both os.Environ() variables and
// the action's built-in variables are visible to interpolation).
//
// Entries are expanded sequentially and each expanded pair is added to the
// resolution scope for the ones that follow. As the config parser flattens the
// root -> project -> action hierarchy into this single ordered list, this makes
// every level able to reference (and, last-wins, override) the levels above it.
//
// Only parameter expansion is performed: command substitution ($(...)) yields
// an error and globbing is disabled, so config values can't spawn processes or
// touch the filesystem.
func expandEnvEntries(entries []string, base []string) ([]string, error) {
	if len(entries) == 0 {
		return nil, nil
	}

	// scope holds everything visible to interpolation: process env, then the
	// built-ins (base) which override it, then each entry as it gets expanded.
	// ListEnviron uses the last value for a duplicate key, so this ordering
	// yields the expected override precedence.
	scope := append(os.Environ(), base...)
	parser := syntax.NewParser()

	out := make([]string, 0, len(entries))
	for _, entry := range entries {
		key, rawVal, found := strings.Cut(entry, "=")
		if !found {
			// Shape is validated at config load; guard anyway.
			return nil, fmt.Errorf("invalid environment entry %q: expected \"KEY=VALUE\" form", entry)
		}
		word, err := parser.Document(strings.NewReader(rawVal))
		if err != nil {
			return nil, fmt.Errorf("environment entry %q: %w", key, err)
		}
		// CmdSubst left nil -> $(...) / `...` => expand.UnexpectedCommandError.
		// ReadDir left nil  -> globbing disabled.
		cfg := &expand.Config{Env: expand.ListEnviron(scope...)}
		val, err := expand.Document(cfg, word)
		if err != nil {
			return nil, fmt.Errorf("environment entry %q: %w", key, err)
		}
		pair := key + "=" + val
		out = append(out, pair)
		scope = append(scope, pair)
	}
	return out, nil
}
