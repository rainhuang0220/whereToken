// Package gemini reads Gemini CLI session recordings.
//
// Official schema (google-gemini/gemini-cli chatRecordingTypes.TokensSummary):
// input / output / cached / thoughts / total on type=gemini messages.
// Files live under ~/.gemini/tmp/<project>/chats/session-*.jsonl (or .json).
// oauth_creds.json, settings.json, and message content are not copied onto events.
package gemini

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

func (Adapter) ID() string { return "gemini" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	var out []adapter.SourceRoot
	seen := map[string]struct{}{}
	add := func(dir string) {
		st, err := os.Stat(dir)
		if err != nil || !st.IsDir() {
			return
		}
		if _, ok := seen[dir]; ok {
			return
		}
		seen[dir] = struct{}{}
		out = append(out, adapter.SourceRoot{ID: "gemini", Path: dir})
	}
	// Official paths.ts: GEMINI_CLI_HOME overrides os.homedir(); sessions live in {homedir}/.gemini.
	if env := strings.TrimSpace(os.Getenv("GEMINI_CLI_HOME")); env != "" {
		add(filepath.Join(env, ".gemini"))
		add(env)
	}
	add(home.DotDir("gemini"))
	return out
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	tmp := filepath.Join(root.Path, "tmp")
	if st, err := os.Stat(tmp); err != nil || !st.IsDir() {
		return nil
	}
	var first error
	err := filepath.WalkDir(tmp, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		name := d.Name()
		if d.IsDir() {
			return nil
		}
		if !underChats(path) {
			return nil
		}
		if !strings.HasSuffix(name, ".jsonl") && !strings.HasSuffix(name, ".json") {
			return nil
		}
		if e := parseSession(path, root, emit, emitTurn); e != nil && first == nil {
			first = e
		}
		return nil
	})
	if err != nil {
		return err
	}
	return first
}

func parseSession(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	if strings.HasSuffix(path, ".json") && !strings.HasSuffix(path, ".jsonl") {
		evs, turns, _, err := index.LoadOrReplay("gemini", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
			return parseJSONFile(f, path, root)
		})
		return index.Forward(evs, turns, err, emit, emitTurn)
	}
	evs, turns, _, err := index.LoadOrParse("gemini", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseJSONLFile(f, path, root)
	})
	return index.Forward(evs, turns, err, emit, emitTurn)
}

type tokens struct {
	Input    int64 `json:"input"`
	Output   int64 `json:"output"`
	Cached   int64 `json:"cached"`
	Thoughts int64 `json:"thoughts"`
}

type rec struct {
	Type      string          `json:"type"`
	ID        string          `json:"id"`
	Timestamp string          `json:"timestamp"`
	Model     string          `json:"model"`
	Tokens    *tokens         `json:"tokens"`
	SessionID string          `json:"sessionId"`
	Messages  json.RawMessage `json:"messages"`
}

func parseJSONLFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	sess := sessionFromPath(path)
	consumed, err := index.ScanJSONL(f, func(raw []byte, at int64) error {
		if len(raw) == 0 {
			return nil
		}
		var r rec
		if json.Unmarshal(raw, &r) != nil {
			return nil
		}
		if r.SessionID != "" {
			sess = r.SessionID
		}
		e, t, ok := emitFromRec(r, path, root, sess, at)
		if !ok {
			return nil
		}
		if t.Source != "" {
			turns = append(turns, t)
		}
		if e.Source != "" {
			evs = append(evs, e)
		}
		return nil
	})
	return evs, turns, consumed, err
}

func parseJSONFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	st, err := f.Stat()
	if err != nil {
		return nil, nil, 0, err
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, st.Size(), err
	}
	var wrap struct {
		SessionID string `json:"sessionId"`
		Messages  []rec  `json:"messages"`
	}
	if json.Unmarshal(raw, &wrap) != nil {
		return nil, nil, st.Size(), nil
	}
	sess := wrap.SessionID
	if sess == "" {
		sess = sessionFromPath(path)
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	for i, r := range wrap.Messages {
		e, t, ok := emitFromRec(r, path, root, sess, int64(i))
		if !ok {
			continue
		}
		if t.Source != "" {
			turns = append(turns, t)
		}
		if e.Source != "" {
			evs = append(evs, e)
		}
	}
	return evs, turns, st.Size(), nil
}

func emitFromRec(r rec, path string, root adapter.SourceRoot, sess string, at int64) (event.UsageEvent, event.TurnEvent, bool) {
	ts := parseTS(r.Timestamp)
	if r.Type == "user" {
		return event.UsageEvent{}, event.TurnEvent{Source: "gemini", SessionID: sess, Timestamp: ts}, true
	}
	if r.Type != "gemini" || r.Tokens == nil {
		return event.UsageEvent{}, event.TurnEvent{}, false
	}
	in := clamp0(r.Tokens.Input)
	cached := clamp0(r.Tokens.Cached)
	out := clamp0(r.Tokens.Output)
	thoughts := clamp0(r.Tokens.Thoughts)
	miss := in - cached
	if miss < 0 {
		miss = 0
	}
	if miss+cached+out+thoughts == 0 {
		return event.UsageEvent{}, event.TurnEvent{}, false
	}
	req := strings.TrimSpace(r.ID)
	if req == "" {
		req = fmt.Sprintf("%s:%d", path, at)
	}
	return event.UsageEvent{
		Source:     "gemini",
		Vendor:     vendor.Lookup(r.Model, "google"),
		SourceRoot: root.Path,
		RequestID:  req,
		SessionID:  sess,
		Model:      r.Model,
		Provider:   "google",
		Timestamp:  ts,
		Miss:       miss,
		CacheRead:  cached,
		// Official TokensSummary.total = input + output + thoughts. Gemini
		// bills thinking as output (Codex-style fold). Reasoning is stored
		// for display and must not be added into Total again.
		Output:     out + thoughts,
		Reasoning:  thoughts,
		Quality:    event.QualityAuthoritative,
		Derivation: event.DeriveDerived,
	}, event.TurnEvent{}, true
}

func underChats(path string) bool {
	dir := filepath.Dir(path)
	if filepath.Base(dir) == "chats" {
		return true
	}
	// Subagent sessions: tmp/<project>/chats/<parentSessionId>/<id>.jsonl
	return filepath.Base(filepath.Dir(dir)) == "chats"
}

func sessionFromPath(path string) string {
	base := filepath.Base(path)
	base = strings.TrimSuffix(base, ".jsonl")
	base = strings.TrimSuffix(base, ".json")
	return strings.TrimPrefix(base, "session-")
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

func (Adapter) Probe(root adapter.SourceRoot) adapter.Probe {
	return adapter.InferProbe(true, hasChatLedger(root.Path), adapter.Caps{
		Discovery: adapter.LevelYes, Usage: adapter.LevelYes,
		Model: adapter.LevelYes, Timestamp: adapter.LevelYes, Session: adapter.LevelYes,
		Cache: adapter.LevelYes, Reasoning: adapter.LevelYes, Incremental: adapter.LevelYes,
	})
}

func hasChatLedger(root string) bool {
	tmp := filepath.Join(root, "tmp")
	found := false
	_ = filepath.WalkDir(tmp, func(path string, d fs.DirEntry, err error) error {
		if err != nil || d.IsDir() {
			return nil
		}
		if !underChats(path) {
			return nil
		}
		name := d.Name()
		if strings.HasSuffix(name, ".jsonl") || strings.HasSuffix(name, ".json") {
			found = true
			return fs.SkipAll
		}
		return nil
	})
	return found
}
