// Package proclock is a cross-process exclusive lock beside the config
// directory. Profile refresh and another scanner take it so two index
// writers do not run at once. The lock file is empty: it is not a watermark
// and it stores no paths, tokens, or snapshots.
package proclock

import (
	"errors"
	"io"
	"os"
	"path/filepath"
)

// errBusy means another process holds the lock.
var errBusy = errors.New("lock busy")

// Unlock releases the lock and closes the underlying file.
type Unlock func()

// Lock blocks until the exclusive lock is acquired.
func Lock(path string) (Unlock, error) {
	_, rel, err := acquire(path, false)
	return rel, err
}

// TryLock acquires the lock or reports that it is held. A held lock is
// ok == false and a nil error.
func TryLock(path string) (Unlock, bool, error) {
	_, rel, ok, err := tryAcquire(path)
	return rel, ok, err
}

// tryAcquire is TryLock plus the file handle that owns the lock. Windows
// LockFileEx is mandatory, so a second os.ReadFile fails while the lock
// is held. Callers that need the bytes read this handle.
func tryAcquire(path string) (*os.File, Unlock, bool, error) {
	f, rel, err := acquire(path, true)
	if errors.Is(err, errBusy) {
		return nil, nil, false, nil
	}
	if err != nil {
		return nil, nil, false, err
	}
	return f, rel, true, nil
}

func readLocked(f *os.File) ([]byte, error) {
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return nil, err
	}
	return io.ReadAll(f)
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
