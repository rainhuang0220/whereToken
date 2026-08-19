// Package qwen reads Qwen Code's dedicated token-usage JSONL.
//
// Official ledger (QwenLM/qwen-code tokenUsageService):
// ~/.qwen/usage/token-usage-*.jsonl
// Fields: inputTokens, outputTokens, cachedTokens, thoughtsTokens, totalTokens,
// model, sessionId, timestamp. No prompts. Skip settings.json and oauth_creds.json.
package qwen

import (
	"encoding/json"
	"fmt"
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

func (Adapter) ID() string { return "qwen" }

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
		out = append(out, adapter.SourceRoot{ID: "qwen", Path: dir})
	}
	if env := strings.TrimSpace(os.Getenv("QWEN_RUNTIME_DIR")); env != "" {
		add(env)
	}
	if env := strings.TrimSpace(os.Getenv("QWEN_HOME")); env != "" {
		add(env)
	}
	add(home.DotDir("qwen"))
	return out
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	dir := filepath.Join(root.Path, "usage")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	var first error
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		name := e.Name()
		if !strings.HasPrefix(name, "token-usage-") || !strings.HasSuffix(name, ".jsonl") {
			continue
		}
		if err := parseFile(filepath.Join(dir, name), root, emit); err != nil && first == nil {
			first = err
		}
	}
	return first
}

type rec struct {
	ID             string `json:"id"`
	Timestamp      string `json:"timestamp"`
	SessionID      string `json:"sessionId"`
	Model          string `json:"model"`
	InputTokens    int64  `json:"inputTokens"`
	OutputTokens   int64  `json:"outputTokens"`
	CachedTokens   int64  `json:"cachedTokens"`
	ThoughtsTokens int64  `json:"thoughtsTokens"`
}

func parseFile(path string, root adapter.SourceRoot, emit func(event.UsageEvent)) error {
	evs, _, _, err := index.LoadOrParse("qwen", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseJSONL(f, path, root)
	})
	if err != nil {
		return err
	}
	for _, e := range evs {
		emit(e)
	}
	return nil
}

func parseJSONL(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	var evs []event.UsageEvent
	consumed, err := index.ScanJSONL(f, func(raw []byte, at int64) error {
		if len(raw) == 0 {
			return nil
		}
		var r rec
		if json.Unmarshal(raw, &r) != nil {
			return nil
		}
		in := clamp0(r.InputTokens)
		cached := clamp0(r.CachedTokens)
		out := clamp0(r.OutputTokens)
		thoughts := clamp0(r.ThoughtsTokens)
		miss := in - cached
		if miss < 0 {
			miss = 0
		}
		if miss+cached+out == 0 {
			return nil
		}
		req := strings.TrimSpace(r.ID)
		if req == "" {
			req = fmt.Sprintf("%s:%d", path, at)
		}
		evs = append(evs, event.UsageEvent{
			Source:     "qwen",
			Vendor:     vendor.Lookup(r.Model, "dashscope"),
			SourceRoot: root.Path,
			RequestID:  req,
			SessionID:  r.SessionID,
			Model:      r.Model,
			Provider:   "dashscope",
			Timestamp:  parseTS(r.Timestamp),
			Miss:       miss,
			CacheRead:  cached,
			Output:     out,
			Reasoning:  thoughts,
			Quality:    event.QualityAuthoritative,
			Derivation: event.DeriveDerived,
		})
		return nil
	})
	return evs, nil, consumed, err
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
	return adapter.InferProbe(true, hasUsageLedger(root.Path), adapter.Caps{
		Discovery: adapter.LevelYes, Usage: adapter.LevelYes,
		Model: adapter.LevelYes, Timestamp: adapter.LevelYes, Session: adapter.LevelYes,
		Cache: adapter.LevelYes, Reasoning: adapter.LevelYes, Incremental: adapter.LevelYes,
	})
}

func hasUsageLedger(root string) bool {
	dir := filepath.Join(root, "usage")
	entries, err := os.ReadDir(dir)
	if err != nil {
		return false
	}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() && strings.HasPrefix(name, "token-usage-") && strings.HasSuffix(name, ".jsonl") {
			return true
		}
	}
	return false
}
