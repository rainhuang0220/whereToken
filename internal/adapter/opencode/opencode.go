package opencode

import (
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

type Adapter struct{}

func (Adapter) ID() string { return "opencode" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	dir := home.XDGData("opencode")
	if _, ok := dbFile(dir); ok {
		return []adapter.SourceRoot{{ID: "opencode", Path: dir}}
	}
	return nil
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	path, ok := dbFile(root.Path)
	if !ok {
		return nil
	}
	return parseDB(path, root, emit, emitTurn)
}

func dbFile(path string) (string, bool) {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		return path, true
	}
	for _, name := range []string{"opencode.db", "opencode-stable.db"} {
		p := filepath.Join(path, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func parseDB(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrReplay("opencode", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return adapter.ParseOpenDB("opencode", f.Name(), root)
	})
	return index.Forward(evs, turns, err, emit, emitTurn)
}
