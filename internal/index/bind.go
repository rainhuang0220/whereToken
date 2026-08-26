package index

import (
	"fmt"
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

// Forward emits cached or newly parsed events even when parse failed, so a
// later incremental error cannot drop tokens the store already returned.
func Forward(evs []event.UsageEvent, turns []event.TurnEvent, err error, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	for _, e := range evs {
		emit(e)
	}
	if emitTurn != nil {
		for _, t := range turns {
			emitTurn(t)
		}
	}
	return err
}

func parseFull(path string, parse ParseFunc) ([]event.UsageEvent, []event.TurnEvent, string, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, nil, ModeFull, fmt.Errorf("open: %w", PathFree(err))
	}
	evs, turns, _, err := parse(f)
	f.Close()
	return evs, turns, ModeFull, err
}
