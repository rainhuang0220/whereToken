package main

import (
	"os"

	"github.com/mattn/go-isatty"
	"github.com/rainhuang0220/whereToken/internal/cli"
)

// set by goreleaser / -ldflags
var version = "dev"

func main() {
	app := cli.App{
		Args:      os.Args[1:],
		Stdout:    os.Stdout,
		Stderr:    os.Stderr,
		Version:   version,
		StdoutTTY: isatty.IsTerminal(os.Stdout.Fd()),
	}
	os.Exit(app.Run())
}
