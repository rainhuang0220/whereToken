package index

import (
	"os"
	"sync"

	"github.com/rainhuang0220/whereToken/internal/event"
)

var (
	bindMu sync.Mutex
	active *Store
)

func Use(s *Store) func() {
	bindMu.Lock()
	prev := active
	active = s
	bindMu.Unlock()
	return func() {
		bindMu.Lock()
		active = prev
		bindMu.Unlock()
	}
}

func Active() *Store {
	bindMu.Lock()
	s := active
	bindMu.Unlock()
	return s
}

func LoadOrParse(source, path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	if s := Active(); s != nil {
		return s.LoadOrParse(source, path, parse)
	}
	return parseFull(path, parse)
}

func LoadOrReplay(source, path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	if s := Active(); s != nil {
		return s.LoadOrReplay(source, path, parse)
	}
	return parseFull(path, parse)
}

func parseFull(path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, ModeFull, err
	}
	evs, turns, _, err := parse(f)
	f.Close()
	return evs, turns, ModeFull, err
}
