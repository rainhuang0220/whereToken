package openclaw

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

// Adapter reads OpenClaw session JSONL usage.
//
// Only agents/*/sessions/*.jsonl (not *.trajectory.jsonl). Login files,
// credentials, workspace trees, and message bodies are ignored.
type Adapter struct{}

func (Adapter) ID() string { return "openclaw" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	dir := home.DotDir("openclaw")
	st, err := os.Stat(dir)
	if err != nil || !st.IsDir() {
		return nil
	}
	return []adapter.SourceRoot{{ID: "openclaw", Path: dir}}
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	agents := filepath.Join(root.Path, "agents")
	return filepath.WalkDir(agents, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "credentials", "workspace", "skills-prompts", "plugins":
				return fs.SkipDir
			}
			return nil
		}
		name := d.Name()
		if !strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".trajectory.jsonl") {
			return nil
		}
		return parseSession(path, root, emit, emitTurn)
	})
}

type line struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp json.RawMessage `json:"timestamp"`
	Cwd       string          `json:"cwd"`
	Message   json.RawMessage `json:"message"`
}

type msg struct {
	Role       string `json:"role"`
	Provider   string `json:"provider"`
	Model      string `json:"model"`
	ResponseID string `json:"responseId"`
	Usage      *struct {
		Input      int64 `json:"input"`
		Output     int64 `json:"output"`
		CacheRead  int64 `json:"cacheRead"`
		CacheWrite int64 `json:"cacheWrite"`
	} `json:"usage"`
}

func parseSession(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrParse("openclaw", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseFile(f, path, root)
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

func parseFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	sess := sessionID(path)
	var workspace string
	consumed, err := index.ScanJSONL(f, func(raw []byte, at int64) error {
		if len(raw) == 0 {
			return nil
		}
		var rec line
		if json.Unmarshal(raw, &rec) != nil {
			return nil
		}
		switch rec.Type {
		case "session":
			if rec.Cwd != "" {
				workspace = rec.Cwd
			}
			if rec.ID != "" {
				sess = rec.ID
			}
		case "message":
			var m msg
			if json.Unmarshal(rec.Message, &m) != nil {
				return nil
			}
			ts := parseTSRaw(rec.Timestamp)
			if m.Role == "user" {
				turns = append(turns, event.TurnEvent{
					Source:    "openclaw",
					SessionID: sess,
					Workspace: workspace,
					Timestamp: ts,
				})
				return nil
			}
			if m.Role != "assistant" || m.Usage == nil {
				return nil
			}
			miss := clamp0(m.Usage.Input)
			out := clamp0(m.Usage.Output)
			cr := clamp0(m.Usage.CacheRead)
			cw := clamp0(m.Usage.CacheWrite)
			if miss+out+cr+cw == 0 {
				return nil
			}
			req := strings.TrimSpace(m.ResponseID)
			if req == "" {
				req = fmt.Sprintf("%s:%d", path, at)
			}
			evs = append(evs, event.UsageEvent{
				Source:      "openclaw",
				Vendor:      vendor.Lookup(m.Model, m.Provider),
				SourceRoot:  root.Path,
				RequestID:   req,
				SessionID:   sess,
				Workspace:   workspace,
				Model:       m.Model,
				Provider:    m.Provider,
				Timestamp:   ts,
				Miss:        miss,
				CacheRead:   cr,
				CacheCreate: cw,
				Output:      out,
				Quality:     event.QualityAuthoritative,
				Derivation:  event.DeriveRaw,
			})
		}
		return nil
	})
	return evs, turns, consumed, err
}

func sessionID(path string) string {
	base := filepath.Base(path)
	return strings.TrimSuffix(base, ".jsonl")
}

func parseTSRaw(raw json.RawMessage) time.Time {
	if len(raw) == 0 {
		return time.Time{}
	}
	var s string
	if json.Unmarshal(raw, &s) == nil {
		return parseTS(s)
	}
	var n int64
	if json.Unmarshal(raw, &n) == nil && n > 0 {
		if n > 1e12 {
			return time.UnixMilli(n).UTC()
		}
		return time.Unix(n, 0).UTC()
	}
	return time.Time{}
}

func parseTS(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	if t, err := time.Parse(time.RFC3339Nano, s); err == nil {
		return t.UTC()
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.UTC()
	}
	return time.Time{}
}

func clamp0(n int64) int64 {
	if n < 0 {
		return 0
	}
	return n
}
