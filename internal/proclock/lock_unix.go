//go:build !windows

package proclock

import (
	"errors"
	"os"

	"golang.org/x/sys/unix"
)

func acquire(path string, nonblock bool) (*os.File, Unlock, error) {
	f, err := openLock(path)
	if err != nil {
		return nil, nil, err
	}
	flags := unix.LOCK_EX
	if nonblock {
		flags |= unix.LOCK_NB
	}
	if err := unix.Flock(int(f.Fd()), flags); err != nil {
		_ = f.Close()
		if errors.Is(err, unix.EWOULDBLOCK) || errors.Is(err, unix.EAGAIN) {
			return nil, nil, errBusy
		}
		return nil, nil, err
	}
	return f, func() {
		_ = unix.Flock(int(f.Fd()), unix.LOCK_UN)
		_ = f.Close()
	}, nil
}
