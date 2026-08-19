// Package kilo reads Kilo Code usage from leftover VS Code task metrics
// and from the Kilo CLI 1.x OpenCode-shaped kilo.db.
//
// Marketplace leftover: kilocode.kilo-code ui_messages.json (api_req_started).
// CLI: ~/.local/share/kilo/kilo.db message.data.tokens. Skip auth.json,
// settings/, and OpenCode's opencode.db (a different tool).
package kilo

import (
	"os"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/vsctask"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct{}

func (Adapter) ID() string { return "kilo" }

func vscodeOpts() vsctask.Options {
	return vsctask.Options{
		SourceID: "kilo",
		ExtIDs: []string{
			"kilocode.kilo-code",
			"kilocode.Kilo-Code",
		},
		Says: map[string]bool{"api_req_started": true},
	}
}

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	out := vsctask.Discover(home, vscodeOpts())
	if env := strings.TrimSpace(os.Getenv("KILO_DB")); env != "" {
		if path, ok := dbFile(env); ok {
			out = append(out, adapter.SourceRoot{ID: "kilo", Path: path})
		}
	}
	if dir := home.XDGData("kilo"); dir != "" {
		if _, ok := dbFile(dir); ok {
			out = append(out, adapter.SourceRoot{ID: "kilo", Path: dir})
		}
	}
	return out
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	if path, ok := dbFile(root.Path); ok {
		return parseDB(path, root, emit, emitTurn)
	}
	return vsctask.Parse(root, vscodeOpts(), emit, emitTurn)
}

func (Adapter) Probe(root adapter.SourceRoot) adapter.Probe {
	if _, ok := dbFile(root.Path); ok {
		return adapter.InferProbe(true, true, adapter.Caps{
			Discovery: adapter.LevelYes, Usage: adapter.LevelYes,
			Cache: adapter.LevelYes, Incremental: adapter.LevelUnavailable,
		})
	}
	return vsctask.Probe(root)
}
