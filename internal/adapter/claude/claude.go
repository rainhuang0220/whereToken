package claude

import (
	"bufio"
	"encoding/json"
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
	return filepath.WalkDir(root.Path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if d.IsDir() {
			return nil
		}
		if d.Name() == "settings.json" {
			return nil
		}
		if !strings.HasSuffix(d.Name(), ".jsonl") {
			return nil
		}
		return parseJSONL(path, root, emit, emitTurn)
	})
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
	ws, sess := claudeContext(root.Path, path)
	f, err := os.Open(path)
	if err != nil {
		return err
	}
	defer f.Close()

	sc := bufio.NewScanner(f)
	buf := make([]byte, 0, 64*1024)
	sc.Buffer(buf, 10*1024*1024)
	for sc.Scan() {
		b := sc.Bytes()
		if len(b) == 0 {
			continue
		}
		var rec claudeLine
		if err := json.Unmarshal(b, &rec); err != nil {
			continue
		}
		ts, _ := time.Parse(time.RFC3339, rec.Timestamp)
		switch rec.Type {
		case "assistant":
			if rec.Message.Usage == nil {
				continue
			}
			req := claudeRequestID(rec)
			if req == "" {
				// uuid is unique per JSONL line; using it as RequestID
				// would defeat mergeByRequest and sum stream placeholders.
				continue
			}
			emit(event.UsageEvent{
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
			})
		case "user":
			if isUserTurn(rec.Message.Content) {
				emitTurn(event.TurnEvent{Source: "claude", SessionID: sess, Workspace: ws, Timestamp: ts})
			}
		}
	}
	return sc.Err()
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
