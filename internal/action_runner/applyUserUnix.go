//go:build unix

package action_runner

import (
	"os/user"
	"strconv"
	"syscall"
)

func applyUser(username string) (*syscall.SysProcAttr, error) {
	usr, err := getUnixUserInfo(username)
	if err != nil {
		return nil, err
	}
	sysProcAttr := &syscall.SysProcAttr{}
	sysProcAttr.Credential = &syscall.Credential{Uid: usr.Uid, Gid: usr.Gid}
	return sysProcAttr, nil
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
