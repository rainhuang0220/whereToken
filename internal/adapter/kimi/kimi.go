package kimi

import (
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Adapter struct{}

func (Adapter) ID() string { return "kimi" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	var out []adapter.SourceRoot
	var seen []os.FileInfo
	for _, name := range []string{"kimi-code", "kimi"} {
		p := home.DotDir(name)
		st, err := os.Stat(p)
		if err != nil || !st.IsDir() {
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
		out = append(out, adapter.SourceRoot{ID: "kimi", Path: p})
	}
	return out
}

func (a Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	var first error
	err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			base := d.Name()
			if base == "telemetry" || base == "credentials" {
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "wire.jsonl" {
			return nil
		}
		if e := parseWire(path, root, emit, emitTurn); e != nil && first == nil {
			first = e
		}
		return nil
	})
	if err != nil {
		return err
	}
	return first
}

type wireLine struct {
	Type  string `json:"type"`
	Model string `json:"model"`
	Usage struct {
		InputOther         int64 `json:"inputOther"`
		Output             int64 `json:"output"`
		InputCacheRead     int64 `json:"inputCacheRead"`
		InputCacheCreation int64 `json:"inputCacheCreation"`
	} `json:"usage"`
	Origin struct {
		Kind string `json:"kind"`
	} `json:"origin"`
	Time int64 `json:"time"`
}

func parseWire(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrParse("kimi", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseWireFile(f, path, root)
	})
	if err != nil {
		return err
	}
	for _, e := range evs {
		emit(e)
	}
	for _, t := range turns {
		emitTurn(t)
	}
	return nil
}

func parseWireFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	seq := 0
	consumed, err := index.ScanJSONL(f, func(line []byte, at int64) error {
		if len(line) == 0 {
			return nil
		}
		var rec wireLine
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil
		}
		switch rec.Type {
		case "usage.record":
			seq++
			ws, sess := kimiContext(root.Path, path)
			evs = append(evs, event.UsageEvent{
				Source:      "kimi",
				Vendor:      vendor.Lookup(rec.Model, ""),
				SourceRoot:  root.Path,
				RequestID:   fmt.Sprintf("%s:%d:%d:%d", path, rec.Time, seq, at),
				SessionID:   sess,
				Workspace:   ws,
				Model:       rec.Model,
				Timestamp:   time.UnixMilli(rec.Time).UTC(),
				Miss:        rec.Usage.InputOther,
				CacheRead:   rec.Usage.InputCacheRead,
				CacheCreate: rec.Usage.InputCacheCreation,
				Output:      rec.Usage.Output,
				Quality:     event.QualityAuthoritative,
				Derivation:  event.DeriveRaw,
			})
		case "turn.prompt":
			if rec.Origin.Kind == "user" {
				ws, sess := kimiContext(root.Path, path)
				turns = append(turns, event.TurnEvent{
					Source:    "kimi",
					SessionID: sess,
					Workspace: ws,
					Timestamp: time.UnixMilli(rec.Time).UTC(),
				})
			}
		}
		return nil
	})
	return evs, turns, consumed, err
}

func kimiContext(rootPath, file string) (workspace, session string) {
	rel, err := filepath.Rel(rootPath, file)
	if err != nil {
		rel = file
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	for i, p := range parts {
		if p != "agents" || i < 1 {
			continue
		}
		session = parts[i-1]
		if i >= 2 {
			workspace = parts[i-2]
		}
		return workspace, session
	}
	return "", ""
}
