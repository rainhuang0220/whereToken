// Package cline reads Cline task metrics from ui_messages.json.
//
// Official getApiMetrics (cline/cline) sums tokensIn / tokensOut / cacheReads /
// cacheWrites from say=api_req_started (also deleted_api_reqs, subagent_usage).
// tokensIn is a separate bucket from cache columns. settings/ and full
// transcripts are not read. cost in those blobs is ignored.
package cline

import (
	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/vsctask"
	"github.com/rainhuang0220/whereToken/internal/event"
)

type Adapter struct{}

func (Adapter) ID() string { return "cline" }

func opts() vsctask.Options {
	return vsctask.Options{
		SourceID: "cline",
		ExtIDs:   []string{"saoudrizwan.claude-dev"},
		Says: map[string]bool{
			"api_req_started":  true,
			"deleted_api_reqs": true,
			"subagent_usage":   true,
		},
	}
}

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	return vsctask.Discover(home, opts())
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	return vsctask.Parse(root, opts(), emit, emitTurn)
}
