//go:build !unix

package actionrunner

import "syscall"

// chownActionDir is a no-op on non-unix platforms: the `user` field is
// unsupported there (getSysProcAttr rejects it), so a runner-created directory
// always belongs to the receiver's user, which is who the action runs as.
func chownActionDir(_ string, _ *syscall.SysProcAttr) error {
	return nil
}
