//go:build unix

package actionrunner

import (
	"os"
	"syscall"
)

// chownActionDir hands ownership of a runner-created directory to the user the
// action runs as, so a setuid'd action can write into an otherwise 0700,
// receiver-owned directory. It's a no-op when no `user` override is configured
// (Credential is nil): the directory already belongs to the receiver's own
// user, which is who the action runs as.
func chownActionDir(dir string, sysProcAttr *syscall.SysProcAttr) error {
	if sysProcAttr == nil || sysProcAttr.Credential == nil {
		return nil
	}
	cred := sysProcAttr.Credential
	return os.Chown(dir, int(cred.Uid), int(cred.Gid))
}
