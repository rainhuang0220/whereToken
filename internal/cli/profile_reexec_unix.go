//go:build unix

package cli

import (
	"os"
	"syscall"
)

func (a *App) reexec(program string, argv []string) {
	if a.Reexec != nil {
		a.Reexec(argv)
		return
	}
	_ = syscall.Exec(program, argv, os.Environ())
}
