package cursor

import (
	"os"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct{}

func (Adapter) ID() string { return "cursor" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	p := home.DotDir("cursor")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return []adapter.SourceRoot{{ID: "cursor", Path: p}}
	}
	return nil
}

func (Adapter) Parse(adapter.SourceRoot, func(event.UsageEvent), func(event.TurnEvent)) error {
	return nil
}
