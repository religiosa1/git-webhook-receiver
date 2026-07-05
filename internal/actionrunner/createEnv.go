package actionrunner

import (
	"fmt"
	"os"
)

// passthroughEnv lists parent-process variables forwarded into the otherwise
// isolated action environment. PATH lets actions resolve binaries by name;
// HOME/USER let tools like git find their config and identity.
var passthroughEnv = []string{"PATH", "HOME", "USER"}

// createEnv creates an environment for the action runner, where each entry is in
// the form of "key=value"
func createEnv(args ActionArgs) []string {
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
	}
	for _, key := range passthroughEnv {
		if val, ok := os.LookupEnv(key); ok {
			env = append(env, fmt.Sprintf("%s=%s", key, val))
		}
	}
	return env
}
