package adapter

import "github.com/rainhuang0220/whereToken/internal/event"

type SourceRoot struct {
	ID   string
	Path string
}

type Home interface {
	DotDir(name string) string     // $HOME/.<name>
	XDGData(name string) string    // $XDG_DATA_HOME/<name> or $HOME/.local/share/<name>
	AppSupport(name string) string // macOS Application Support/<name>
}

type Adapter interface {
	ID() string
	Discover(home Home) []SourceRoot
	Parse(root SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error
}
