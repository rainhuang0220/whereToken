package index

import (
	"errors"
	"io/fs"
)

// PathFree drops the path an *fs.PathError carries so user-visible errors
// cannot leak filesystem layout. The underlying errno survives, so
// errors.Is/As on syscall errors still works.
func PathFree(err error) error {
	var pe *fs.PathError
	if errors.As(err, &pe) {
		return pe.Err
	}
	return err
}
