//go:build unix

package actionrunner

import (
	"os"
	"syscall"
	"testing"
)

// chownActionDir must not touch the directory when no user override is set --
// the dir already belongs to the receiver's user. Actually chowning to a
// different user requires root, so only the no-op paths are exercised here.
func TestChownActionDirNoUserIsNoop(t *testing.T) {
	dir := t.TempDir()
	before, err := os.Stat(dir)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}

	for name, attr := range map[string]*syscall.SysProcAttr{
		"nil sysProcAttr": nil,
		"nil Credential":  {Setpgid: true},
	} {
		t.Run(name, func(t *testing.T) {
			if err := chownActionDir(dir, attr); err != nil {
				t.Fatalf("chownActionDir returned error: %v", err)
			}
			after, err := os.Stat(dir)
			if err != nil {
				t.Fatalf("stat: %v", err)
			}
			beforeSys := before.Sys().(*syscall.Stat_t)
			afterSys := after.Sys().(*syscall.Stat_t)
			if beforeSys.Uid != afterSys.Uid || beforeSys.Gid != afterSys.Gid {
				t.Errorf("ownership changed: uid %d->%d gid %d->%d",
					beforeSys.Uid, afterSys.Uid, beforeSys.Gid, afterSys.Gid)
			}
		})
	}
}
