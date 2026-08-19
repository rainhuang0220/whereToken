package grok

import (
	"encoding/json"
	"io/fs"
	"net/url"
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

func (Adapter) ID() string { return "grok" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	p := filepath.Join(home.DotDir("grok"), "sessions")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return []adapter.SourceRoot{{ID: "grok", Path: p}}
	}
	return nil
}

func (a Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	var first error
	err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			switch d.Name() {
			case "terminal", "assets", "compaction", "compaction_checkpoints",
				"compaction_requests", "recap_requests", "subagents", "workflows",
				"images", "goal":
				return fs.SkipDir
			}
			return nil
		}
		if d.Name() != "updates.jsonl" {
			return nil
		}
		if e := parseUpdates(path, root, emit, emitTurn); e != nil && first == nil {
			first = e
		}
		return nil
	})
	if err != nil {
		return err
	}
	return first
}

type grokUsage struct {
	InputTokens         int64                `json:"inputTokens"`
	OutputTokens        int64                `json:"outputTokens"`
	CachedReadTokens    int64                `json:"cachedReadTokens"`
	CacheCreationTokens int64                `json:"cacheCreationTokens"`
	ReasoningTokens     int64                `json:"reasoningTokens"`
	ModelUsage          map[string]grokUsage `json:"modelUsage"`
}

type updateLine struct {
	Timestamp int64 `json:"timestamp"`
	Params    struct {
		SessionID string `json:"sessionId"`
		Update    struct {
			SessionUpdate string     `json:"sessionUpdate"`
			PromptID      string     `json:"prompt_id"`
			Usage         *grokUsage `json:"usage"`
			Content       struct {
				Type string `json:"type"`
				Text string `json:"text"`
			} `json:"content"`
		} `json:"update"`
		Meta struct {
			AgentTimestampMs int64 `json:"agentTimestampMs"`
		} `json:"_meta"`
	} `json:"params"`
}

func parseUpdates(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrParse("grok", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseUpdatesFile(f, path, root)
	})
	return index.Forward(evs, turns, err, emit, emitTurn)
}

func parseUpdatesFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	ws, sess := grokContext(root.Path, path)
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	consumed, err := index.ScanJSONL(f, func(line []byte, _ int64) error {
		if len(line) == 0 {
			return nil
		}
		var rec updateLine
		if err := json.Unmarshal(line, &rec); err != nil {
			return nil
		}
		ts := grokTime(rec)
		if rec.Params.SessionID != "" {
			sess = rec.Params.SessionID
		}
		switch rec.Params.Update.SessionUpdate {
		case "user_message_chunk":
			if !isUserTurn(rec.Params.Update.Content.Text) {
				return nil
			}
			turns = append(turns, event.TurnEvent{
				Source:    "grok",
				SessionID: sess,
				Workspace: ws,
				Timestamp: ts,
			})
		case "turn_completed":
			evs = append(evs, completedEvents(rec, root, ws, sess, ts)...)
		}
		return nil
	})
	return evs, turns, consumed, err
}

func grokRequestID(sess, prompt, model string, split bool) string {
	id := prompt
	if sess != "" {
		id = sess + ":" + prompt
	}
	if split && model != "" {
		id += ":" + model
	}
	return id
}

func completedEvents(rec updateLine, root adapter.SourceRoot, ws, sess string, ts time.Time) []event.UsageEvent {
	u := rec.Params.Update.Usage
	if u == nil {
		return nil
	}
	prompt := strings.TrimSpace(rec.Params.Update.PromptID)
	if prompt == "" {
		// eventId is unique per JSONL line; using it would defeat mergeByRequest.
		return nil
	}
	if len(u.ModelUsage) == 0 {
		return []event.UsageEvent{usageEvent(root, ws, sess, grokRequestID(sess, prompt, "", false), "grok", ts, *u)}
	}
	var out []event.UsageEvent
	if len(u.ModelUsage) == 1 {
		for model, part := range u.ModelUsage {
			out = append(out, usageEvent(root, ws, sess, grokRequestID(sess, prompt, model, false), model, ts, part))
		}
		return out
	}
	for model, part := range u.ModelUsage {
		out = append(out, usageEvent(root, ws, sess, grokRequestID(sess, prompt, model, true), model, ts, part))
	}
	return out
}

func usageEvent(root adapter.SourceRoot, ws, sess, req, model string, ts time.Time, u grokUsage) event.UsageEvent {
	return event.UsageEvent{
		Source:      "grok",
		Vendor:      vendor.Lookup(model, ""),
		SourceRoot:  root.Path,
		RequestID:   req,
		SessionID:   sess,
		Workspace:   ws,
		Model:       model,
		Timestamp:   ts,
		Miss:        missTokens(u),
		CacheRead:   u.CachedReadTokens,
		CacheCreate: u.CacheCreationTokens,
		Output:      u.OutputTokens,
		Reasoning:   u.ReasoningTokens,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveDerived,
	}
}

func missTokens(u grokUsage) int64 {
	m := u.InputTokens - u.CachedReadTokens - u.CacheCreationTokens
	if m < 0 {
		return 0
	}
	return m
}

func isUserTurn(text string) bool {
	text = strings.TrimSpace(text)
	if text == "" {
		return false
	}
	return !strings.HasPrefix(text, "<system-reminder>")
}

func grokTime(rec updateLine) time.Time {
	if rec.Params.Meta.AgentTimestampMs > 0 {
		return time.UnixMilli(rec.Params.Meta.AgentTimestampMs).UTC()
	}
	ts := rec.Timestamp
	if ts >= 1_000_000_000_000 {
		return time.UnixMilli(ts).UTC()
	}
	if ts > 0 {
		return time.Unix(ts, 0).UTC()
	}
	return time.Time{}
}

func grokContext(rootPath, file string) (workspace, session string) {
	rel, err := filepath.Rel(rootPath, file)
	if err != nil {
		rel = file
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) < 2 {
		return "", ""
	}
	session = parts[len(parts)-2]
	raw := strings.Join(parts[:len(parts)-2], "/")
	if dec, err := url.PathUnescape(raw); err == nil {
		return dec, session
	}
	return raw, session
}
