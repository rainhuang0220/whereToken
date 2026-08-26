// Package roo reads Roo Code task metrics from ui_messages.json.
//
// Official consolidateTokenUsage (RooCodeInc/Roo-Code) sums tokensIn /
// tokensOut / cacheReads / cacheWrites from say=api_req_started.
// Extension id is RooVeterinaryInc.roo-cline (nightly: roo-code-nightly).
// settings/ and conversation bodies are not copied onto events. cost ignored.
package roo

import (
	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/vsctask"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct{}

func (Adapter) ID() string { return "roo" }

func opts() vsctask.Options {
	return vsctask.Options{
		SourceID: "roo",
		ExtIDs: []string{
			"RooVeterinaryInc.roo-cline",
			"rooveterinaryinc.roo-cline",
			"RooVeterinaryInc.roo-code-nightly",
			"rooveterinaryinc.roo-code-nightly",
		},
		Says: map[string]bool{"api_req_started": true},
	}
}

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	return vsctask.Discover(home, opts())
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	return vsctask.Parse(root, opts(), emit, emitTurn)
}
