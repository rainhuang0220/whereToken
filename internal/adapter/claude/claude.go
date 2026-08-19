package claude

import (
	"encoding/json"
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

func (Adapter) ID() string { return "claude" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	p := adapter.FirstDir(
		filepath.Join(home.DotDir("claude"), "projects"),
		filepath.Join(home.XDGConfig("claude"), "projects"),
	)
	if p != "" {
		return []adapter.SourceRoot{{ID: "claude", Path: p}}
	}
	return nil
}

func (Adapter) Parse(root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	var first error
	err := filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			if d.Name() == "feedback-bundles" {
				return fs.SkipDir
			}
			return nil
		}
		switch d.Name() {
		case "settings.json", "stats-cache.json":
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		if e := parseJSONL(path, root, emit, emitTurn); e != nil && first == nil {
			first = e
		}
		return nil
	})
	if err != nil {
		return err
	}
	return first
}

type claudeLine struct {
	Type      string `json:"type"`
	RequestID string `json:"requestId"`
	UUID      string `json:"uuid"`
	Timestamp string `json:"timestamp"`
	Message   struct {
		ID      string          `json:"id"`
		Model   string          `json:"model"`
		Role    string          `json:"role"`
		Content json.RawMessage `json:"content"`
		Usage   *struct {
			InputTokens              int64 `json:"input_tokens"`
			OutputTokens             int64 `json:"output_tokens"`
			CacheReadInputTokens     int64 `json:"cache_read_input_tokens"`
			CacheCreationInputTokens int64 `json:"cache_creation_input_tokens"`
		} `json:"usage"`
	} `json:"message"`
}

func parseJSONL(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseJSONLFile(f, path, root)
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

func parseJSONLFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	ws, sess := claudeContext(root.Path, path)
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	consumed, err := index.ScanJSONL(f, func(b []byte, _ int64) error {
		if len(b) == 0 {
			return nil
		}
		var rec claudeLine
		if err := json.Unmarshal(b, &rec); err != nil {
			return nil
		}
		ts, _ := time.Parse(time.RFC3339, rec.Timestamp)
		switch rec.Type {
		case "assistant":
			if rec.Message.Usage == nil {
				return nil
			}
			req := claudeRequestID(rec)
			if req == "" {
				// uuid is unique per JSONL line; using it as RequestID
				// would defeat mergeByRequest and sum stream placeholders.
				return nil
			}
			evs = append(evs, event.UsageEvent{
				Source:      "claude",
				Vendor:      vendor.Lookup(rec.Message.Model, ""),
				SourceRoot:  root.Path,
				RequestID:   req,
				Model:       rec.Message.Model,
				Workspace:   ws,
				SessionID:   sess,
				Timestamp:   ts,
				Miss:        rec.Message.Usage.InputTokens,
				CacheRead:   rec.Message.Usage.CacheReadInputTokens,
				CacheCreate: rec.Message.Usage.CacheCreationInputTokens,
				Output:      rec.Message.Usage.OutputTokens,
				Quality:     event.QualityDegraded,
				Derivation:  event.DeriveDeduplicated,
			})
		case "user":
			if isUserTurn(rec.Message.Content) {
				turns = append(turns, event.TurnEvent{Source: "claude", SessionID: sess, Workspace: ws, Timestamp: ts})
			}
		}
		return nil
	})
	return evs, turns, consumed, err
}

func claudeRequestID(rec claudeLine) string {
	if rec.RequestID != "" {
		return rec.RequestID
	}
	return rec.Message.ID
}

func isUserTurn(content json.RawMessage) bool {
	if len(content) == 0 {
		return false
	}
	var s string
	if err := json.Unmarshal(content, &s); err == nil {
		return true
	}
	var parts []struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(content, &parts); err != nil {
		return false
	}
	for _, p := range parts {
		if p.Type == "tool_result" {
			return false
		}
	}
	return len(parts) > 0
}

func claudeContext(rootPath, file string) (workspace, session string) {
	session = strings.TrimSuffix(filepath.Base(file), ".jsonl")
	rel, err := filepath.Rel(rootPath, file)
	if err != nil {
		rel = file
	}
	parts := strings.Split(filepath.ToSlash(rel), "/")
	if len(parts) > 0 && parts[0] == "projects" {
		parts = parts[1:]
	}
	if len(parts) > 0 && parts[0] != session+".jsonl" {
		workspace = parts[0]
	}
	return workspace, session
}
