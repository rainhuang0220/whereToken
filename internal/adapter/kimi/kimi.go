package kimi

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Adapter struct{}

func (Adapter) ID() string { return "kimi" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	var out []adapter.SourceRoot
	for _, name := range []string{"kimi-code", "kimi"} {
		p := home.DotDir(name)
		if st, err := os.Stat(p); err == nil && st.IsDir() {
			out = append(out, adapter.SourceRoot{ID: "kimi", Path: p})
		}
	}
	return out
}

func (a Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	return filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
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
		return parseWire(path, root, emit, emitTurn)
	})
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
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)
	for sc.Scan() {
		line := sc.Bytes()
		if len(line) == 0 {
			continue
		}
		var rec wireLine
		if err := json.Unmarshal(line, &rec); err != nil {
			continue
		}
		switch rec.Type {
		case "usage.record":
			ws, sess := kimiContext(root.Path, path)
			emit(event.UsageEvent{
				Source:      "kimi",
				Vendor:      vendor.Lookup(rec.Model, ""),
				SourceRoot:  root.Path,
				RequestID:   fmt.Sprintf("%s:%d", path, rec.Time),
				SessionID:   sess,
				Workspace:   ws,
				Model:       rec.Model,
				Timestamp:   time.UnixMilli(rec.Time).UTC(),
				Miss:        rec.Usage.InputOther,
				CacheRead:   rec.Usage.InputCacheRead,
				CacheCreate: rec.Usage.InputCacheCreation,
				Output:      rec.Usage.Output,
				Quality:     event.QualityAuthoritative,
			})
		case "turn.prompt":
			if rec.Origin.Kind == "user" {
				ws, sess := kimiContext(root.Path, path)
				emitTurn(event.TurnEvent{
					Source:    "kimi",
					SessionID: sess,
					Workspace: ws,
					Timestamp: time.UnixMilli(rec.Time).UTC(),
				})
			}
		}
	}
	return sc.Err()
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
