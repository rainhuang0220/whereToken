package cli

import "golang.org/x/term"

func terminalSize(fd int) (width, height int, err error) {
	return term.GetSize(fd)
}
