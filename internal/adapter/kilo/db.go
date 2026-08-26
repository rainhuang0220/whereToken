package kilo

import (
	"os"
	"path/filepath"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

func dbFile(path string) (string, bool) {
	if st, err := os.Stat(path); err == nil && !st.IsDir() {
		base := filepath.Base(path)
		if base == "kilo.db" || base == "kilo-stable.db" {
			return path, true
		}
		return "", false
	}
	for _, name := range []string{"kilo.db", "kilo-stable.db"} {
		p := filepath.Join(path, name)
		if st, err := os.Stat(p); err == nil && !st.IsDir() {
			return p, true
		}
	}
	return "", false
}

func parseDB(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrReplay("kilo", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return adapter.ParseOpenDB("kilo", f.Name(), root)
	})
	return index.Forward(evs, turns, err, emit, emitTurn)
}
