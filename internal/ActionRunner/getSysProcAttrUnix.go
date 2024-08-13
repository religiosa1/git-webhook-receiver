//go:build unix

package ActionRunner

import (
	"fmt"
	"os/user"
	"strconv"
	"syscall"
)

func getSysProcAttr(username string) (*syscall.SysProcAttr, error) {
	// On Unix-like os, spawned processes inherit the process group of the parent
	// process, which will prevent gracefull shutdown. So we're spawning it in a separate
	// group.
	// https://www.dolthub.com/blog/2022-11-28-go-os-exec-patterns/#process-groups-and-graceful-shutdown
	sysProcAttr := syscall.SysProcAttr{Setpgid: true}

	if username != "" {
		usr, err := getUnixUserInfo(username)
		if err != nil {
			return nil, fmt.Errorf("unable to obtain UID/GID for the specified user '%s', does it exist?: %w", username, err)
		}
		sysProcAttr.Credential = &syscall.Credential{Uid: usr.Uid, Gid: usr.Gid}
	}

	return &sysProcAttr, nil
}

type UnixUserInfo struct {
	Uid uint32
	Gid uint32
}

func getUnixUserInfo(username string) (*UnixUserInfo, error) {
	user, err := user.Lookup(username)
	if err != nil {
		return nil, err
	}
	uid, err := strconv.ParseUint(user.Uid, 10, 32)
	if err != nil {
		return nil, err
	}
	gid, err := strconv.ParseUint(user.Gid, 10, 32)
	if err != nil {
		return nil, err
	}
	return &UnixUserInfo{Uid: uint32(uid), Gid: uint32(gid)}, nil
}
