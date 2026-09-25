// Package proclock is a cross-process exclusive lock beside the config
// directory. Profile refresh and another scanner take it so two index
// writers do not run at once. The lock file is empty: it is not a watermark
// and it stores no paths, tokens, or snapshots.
package proclock

import (
	"errors"
	"os"
	"path/filepath"
)

// errBusy means another process holds the lock.
var errBusy = errors.New("lock busy")

// Unlock releases the lock and closes the underlying file.
type Unlock func()

// Lock blocks until the exclusive lock is acquired.
func Lock(path string) (Unlock, error) {
	return acquire(path, false)
}

// TryLock acquires the lock or reports that it is held. A held lock is
// ok == false and a nil error.
func TryLock(path string) (Unlock, bool, error) {
	rel, err := acquire(path, true)
	if errors.Is(err, errBusy) {
		return nil, false, nil
	}
	if err != nil {
		return nil, false, err
	}
	return rel, true, nil
}

func openLock(path string) (*os.File, error) {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return nil, err
	}
	if err := os.Chmod(dir, 0o700); err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_CREATE|os.O_RDWR, 0o600)
	if err != nil {
		return nil, err
	}
	if err := f.Chmod(0o600); err != nil {
		_ = f.Close()
		return nil, err
	}
	return f, nil
}
