//go:build windows

package cli

func (a *App) reexec(program string, argv []string) {
	if a.Reexec != nil {
		a.Reexec(argv)
	}
}
