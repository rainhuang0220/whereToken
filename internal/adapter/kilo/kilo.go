// Package kilo reads leftover Kilo Code (Cline-fork) task metrics.
//
// Marketplace id kilocode.kilo-code / kilocode.Kilo-Code. Official
// consolidateTokenUsage sums tokensIn/Out cacheReads/Writes from
// say=api_req_started. The rebuilt Kilo CLI 1.x (kilo.db / OpenCode
// fork) is a different product and is not read here.
package kilo

import (
	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/vsctask"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct{}

func (Adapter) ID() string { return "kilo" }

func opts() vsctask.Options {
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
	return vsctask.Discover(home, opts())
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	return vsctask.Parse(root, opts(), emit, emitTurn)
}

func (Adapter) Probe(root adapter.SourceRoot) adapter.Probe {
	return vsctask.Probe(root)
}
