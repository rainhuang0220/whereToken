package adapter

import "github.com/rainhuang0220/whereToken/internal/event"

type SourceRoot struct {
	ID       string
	Path     string
	AuthPath string // optional local session file; never log or commit contents
}

type Home interface {
	DotDir(name string) string     // $HOME/.<name>
	XDGData(name string) string    // $XDG_DATA_HOME/<name> or $HOME/.local/share/<name>
	XDGConfig(name string) string  // $XDG_CONFIG_HOME/<name> or $HOME/.config/<name>
	AppSupport(name string) string // macOS Application Support/<name>
	AppData(name string) string    // Windows %APPDATA%/<name>
}

type Adapter interface {
	ID() string
	Discover(home Home) []SourceRoot
	Parse(root SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error
}
