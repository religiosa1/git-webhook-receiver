//go:build !unix

package ActionRunner

import (
	"errors"
	"syscall"
)

func getSysProcAttr(username string) (*syscall.SysProcAttr, error) {
	if username != "" {
		return nil, errors.New("setting exec user is not supported on non-unix env")
	}
	return nil, nil
}
