package cli

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"runtime"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/card"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func (a *App) runCard(flags Flags, home adapter.Home) int {
	res := a.doScan(home, flags.Quiet, flags.Offline, false)
	sum := metric.AggregateAt(res.Events, res.Turns, a.Now(), a.Loc)
	view := card.NewView(sum, card.StatusOf(sum), a.Version)
	var buf bytes.Buffer
	if err := card.Render(&buf, view); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	if err := replaceCardFile(flags.CardPath, buf.Bytes()); err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitFail
	}
	abs, err := filepath.Abs(flags.CardPath)
	if err != nil {
		abs = flags.CardPath
	}
	fmt.Fprintf(a.Stdout, "wrote %s\n", abs)
	return ExitOK
}

// renameFile is os.Rename, swapped in tests to exercise rollback.
var renameFile = os.Rename

func replaceCardFile(dest string, payload []byte) error {
	if dest == "" {
		return fmt.Errorf("card: missing path")
	}
	if fi, err := os.Stat(dest); err == nil && fi.IsDir() {
		return fmt.Errorf("card: %s is a directory", dest)
	}
	dir := filepath.Dir(dest)
	tmp, err := os.CreateTemp(dir, ".wheretoken-card-*.svg")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	ok := false
	defer func() {
		if !ok {
			os.Remove(tmpName)
		}
	}()
	if _, err := tmp.Write(payload); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmpName, 0o644); err != nil && runtime.GOOS != "windows" {
		return err
	}
	// One rename: POSIX is atomic; Go 1.25 Windows uses
	// MOVEFILE_REPLACE_EXISTING, so dest is never unlinked first. The
	// dest→.old dance is for replacing a running .exe (replaceFile), not SVG.
	if err := renameFile(tmpName, dest); err != nil {
		return err
	}
	ok = true
	return nil
}
