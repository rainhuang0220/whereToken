// Package vsctask reads VS Code-family task metrics JSON used by Cline
// and Roo Code (ui_messages.json say=api_req_started and siblings).
package vsctask

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

const maxMessagesBytes = 256 << 20

var Products = []string{
	"Code", "Code - Insiders", "VSCodium", "Cursor", "Windsurf", "Visual Studio Code",
}

type Options struct {
	SourceID string
	ExtIDs   []string
	Says     map[string]bool
}

func Discover(home adapter.Home, opt Options) []adapter.SourceRoot {
	var out []adapter.SourceRoot
	var seen []os.FileInfo
	for _, extID := range opt.ExtIDs {
		for _, product := range Products {
			dir := adapter.VSCodeExtDir(home, product, extID)
			if dir == "" {
				continue
			}
			st, err := os.Stat(dir)
			if err != nil {
				continue
			}
			dup := false
			for _, prev := range seen {
				if os.SameFile(prev, st) {
					dup = true
					break
				}
			}
			if dup {
				continue
			}
			seen = append(seen, st)
			out = append(out, adapter.SourceRoot{ID: opt.SourceID, Path: dir})
		}
	}
	return out
}

func Parse(root adapter.SourceRoot, opt Options, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	tasks := filepath.Join(root.Path, "tasks")
	entries, err := os.ReadDir(tasks)
	if err != nil {
		return nil
	}
	var first error
	for _, e := range entries {
		if !e.IsDir() {
			continue
		}
		path := filepath.Join(tasks, e.Name(), "ui_messages.json")
		if _, err := os.Stat(path); err != nil {
			continue
		}
		if err := parseMessages(path, e.Name(), root, opt, emit); err != nil && first == nil {
			first = err
		}
	}
	return first
}

func Probe(root adapter.SourceRoot) adapter.Probe {
	tasks := filepath.Join(root.Path, "tasks")
	st, err := os.Stat(tasks)
	ledger := err == nil && st.IsDir()
	c := adapter.Caps{
		Discovery: adapter.LevelYes, Usage: adapter.LevelYes,
		Cache: adapter.LevelYes, Incremental: adapter.LevelUnavailable,
	}
	return adapter.InferProbe(true, ledger, c)
}

func parseMessages(path, session string, root adapter.SourceRoot, opt Options, emit func(event.UsageEvent)) error {
	evs, _, _, err := index.LoadOrReplay(opt.SourceID, path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseJSON(f, path, session, root, opt)
	})
	if err != nil {
		return err
	}
	for _, e := range evs {
		emit(e)
	}
	return nil
}

type msg struct {
	Type string `json:"type"`
	Say  string `json:"say"`
	Text string `json:"text"`
	TS   int64  `json:"ts"`
}

type metrics struct {
	TokensIn    int64  `json:"tokensIn"`
	TokensOut   int64  `json:"tokensOut"`
	CacheWrites int64  `json:"cacheWrites"`
	CacheReads  int64  `json:"cacheReads"`
	Model       string `json:"model"`
}

func parseJSON(f *os.File, path, session string, root adapter.SourceRoot, opt Options) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, nil, 0, err
	}
	if st.Size() > maxMessagesBytes {
		return nil, nil, st.Size(), nil
	}
	dec := json.NewDecoder(f)
	tok, err := dec.Token()
	if err != nil {
		return nil, nil, st.Size(), nil
	}
	delim, ok := tok.(json.Delim)
	if !ok || delim != '[' {
		return nil, nil, st.Size(), nil
	}
	var evs []event.UsageEvent
	i := 0
	for dec.More() {
		var m msg
		if dec.Decode(&m) != nil {
			break
		}
		e, ok := eventFrom(m, i, session, root, opt)
		if ok {
			evs = append(evs, e)
		}
		i++
	}
	return evs, nil, st.Size(), nil
}

func eventFrom(m msg, i int, session string, root adapter.SourceRoot, opt Options) (event.UsageEvent, bool) {
	if m.Type != "say" || !opt.Says[m.Say] {
		return event.UsageEvent{}, false
	}
	var met metrics
	if json.Unmarshal([]byte(m.Text), &met) != nil {
		return event.UsageEvent{}, false
	}
	in := clamp0(met.TokensIn)
	out := clamp0(met.TokensOut)
	cr := clamp0(met.CacheReads)
	cw := clamp0(met.CacheWrites)
	if in+out+cr+cw == 0 {
		return event.UsageEvent{}, false
	}
	return event.UsageEvent{
		Source:      opt.SourceID,
		Vendor:      vendor.Lookup(met.Model, ""),
		SourceRoot:  root.Path,
		RequestID:   fmt.Sprintf("%s:%d:%d", session, m.TS, i),
		SessionID:   session,
		Model:       met.Model,
		Timestamp:   unixMS(m.TS),
		Miss:        in,
		CacheRead:   cr,
		CacheCreate: cw,
		Output:      out,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveRaw,
	}, true
}

func unixMS(n int64) time.Time {
	if n <= 0 {
		return time.Time{}
	}
	if n > 1e12 {
		return time.UnixMilli(n).UTC()
	}
	return time.Unix(n, 0).UTC()
}

func clamp0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
