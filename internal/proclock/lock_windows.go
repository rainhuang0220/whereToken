//go:build windows

package proclock

import (
	"errors"

	"golang.org/x/sys/windows"
)

func acquire(path string, nonblock bool) (Unlock, error) {
	f, err := openLock(path)
	if err != nil {
		return nil, err
	}
	flags := uint32(windows.LOCKFILE_EXCLUSIVE_LOCK)
	if nonblock {
		flags |= windows.LOCKFILE_FAIL_IMMEDIATELY
	}
	var ol windows.Overlapped
	err = windows.LockFileEx(windows.Handle(f.Fd()), flags, 0, 1, 0, &ol)
	if err != nil {
		_ = f.Close()
		if errors.Is(err, windows.ERROR_LOCK_VIOLATION) {
			return nil, errBusy
		}
		return nil, err
	}
	return func() {
		var ol windows.Overlapped
		_ = windows.UnlockFileEx(windows.Handle(f.Fd()), 0, 1, 0, &ol)
		_ = f.Close()
	}, nil
}
