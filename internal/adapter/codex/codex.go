package codex

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
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

func (Adapter) ID() string { return "codex" }

func (Adapter) Discover(home adapter.Home) []adapter.SourceRoot {
	if v := strings.TrimSpace(os.Getenv("CODEX_HOME")); v != "" {
		if st, err := os.Stat(v); err == nil && st.IsDir() {
			return []adapter.SourceRoot{{ID: "codex", Path: v}}
		}
	}
	p := home.DotDir("codex")
	if st, err := os.Stat(p); err == nil && st.IsDir() {
		return []adapter.SourceRoot{{ID: "codex", Path: p}}
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
		if d.Name() == "auth.json" {
			return nil
		}
		if !isRollout(path) {
			return nil
		}
		return parseRollout(path, root, emit, emitTurn)
	})
}

func isRollout(path string) bool {
	base := filepath.Base(path)
	if !strings.HasPrefix(base, "rollout-") || !strings.HasSuffix(base, ".jsonl") {
		return false
	}
	slash := filepath.ToSlash(path)
	return strings.Contains(slash, "/sessions/") || strings.Contains(slash, "/archived_sessions/")
}

type tokenUsage struct {
	InputTokens           int64 `json:"input_tokens"`
	CachedInputTokens     int64 `json:"cached_input_tokens"`
	OutputTokens          int64 `json:"output_tokens"`
	ReasoningOutputTokens int64 `json:"reasoning_output_tokens"`
}

func (u tokenUsage) advances(prev tokenUsage) bool {
	return u.InputTokens > prev.InputTokens ||
		u.CachedInputTokens > prev.CachedInputTokens ||
		u.OutputTokens > prev.OutputTokens ||
		u.ReasoningOutputTokens > prev.ReasoningOutputTokens
}

type rolloutLine struct {
	Timestamp string          `json:"timestamp"`
	Type      string          `json:"type"`
	Payload   json.RawMessage `json:"payload"`
}

type eventPayload struct {
	Type string `json:"type"`
	Info struct {
		Total *tokenUsage `json:"total_token_usage"`
		Last  *tokenUsage `json:"last_token_usage"`
	} `json:"info"`
}

type turnPayload struct {
	Model string `json:"model"`
}

type itemPayload struct {
	Type string `json:"type"`
	Role string `json:"role"`
}

func parseRollout(path string, root adapter.SourceRoot, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) error {
	evs, turns, _, err := index.LoadOrReplay("codex", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return parseRolloutFile(f, path, root)
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

func parseRolloutFile(f *os.File, path string, root adapter.SourceRoot) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
	r := bufio.NewReaderSize(f, 1<<20)
	st := &rolloutState{}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	for {
		b, err := r.ReadBytes('\n')
		if len(b) > 0 {
			if b[len(b)-1] == '\n' {
				b = b[:len(b)-1]
			}
			if len(b) > 0 && b[len(b)-1] == '\r' {
				b = b[:len(b)-1]
			}
			if len(b) > 0 {
				handleRolloutLine(b, path, root, st, func(e event.UsageEvent) {
					evs = append(evs, e)
				}, func(t event.TurnEvent) {
					turns = append(turns, t)
				})
			}
		}
		if err == io.EOF {
			return evs, turns, 0, nil
		}
		if err != nil {
			return evs, turns, 0, err
		}
	}
}

type rolloutState struct {
	prevTotal tokenUsage
	haveTotal bool
	lastSeen  tokenUsage
	haveLast  bool
	model     string
	seq       int
}

func handleRolloutLine(b []byte, path string, root adapter.SourceRoot, st *rolloutState, emit func(event.UsageEvent), emitTurn func(event.TurnEvent)) {
	var rec rolloutLine
	if err := json.Unmarshal(b, &rec); err != nil {
		return
	}
	ts, _ := time.Parse(time.RFC3339, rec.Timestamp)
	switch rec.Type {
	case "turn_context":
		var p turnPayload
		if err := json.Unmarshal(rec.Payload, &p); err == nil && p.Model != "" {
			st.model = p.Model
		}
	case "event_msg":
		var p eventPayload
		if err := json.Unmarshal(rec.Payload, &p); err != nil || p.Type != "token_count" {
			return
		}
		if p.Info.Total != nil {
			cur := *p.Info.Total
			if st.haveTotal && !cur.advances(st.prevTotal) {
				return
			}
			deltaFrom := tokenUsage{}
			if st.haveTotal {
				deltaFrom = st.prevTotal
			}
			emitUsage(path, root, st.model, ts, &st.seq, deltaFrom, cur, emit)
			st.prevTotal = cur
			st.haveTotal = true
			return
		}
		if p.Info.Last != nil {
			cur := *p.Info.Last
			if st.haveLast && cur == st.lastSeen {
				return
			}
			emitUsage(path, root, st.model, ts, &st.seq, tokenUsage{}, cur, emit)
			st.lastSeen = cur
			st.haveLast = true
			if !st.haveTotal {
				st.prevTotal = cur
				st.haveTotal = true
			}
		}
	case "response_item":
		var p itemPayload
		if err := json.Unmarshal(rec.Payload, &p); err != nil {
			return
		}
		if p.Type == "message" && p.Role == "user" {
			emitTurn(event.TurnEvent{
				Source:    "codex",
				SessionID: strings.TrimSuffix(filepath.Base(path), ".jsonl"),
				Timestamp: ts,
			})
		}
	}
}

func emitUsage(path string, root adapter.SourceRoot, model string, ts time.Time, seq *int, prev, cur tokenUsage, emit func(event.UsageEvent)) {
	inDelta := cur.InputTokens - prev.InputTokens
	cachedDelta := cur.CachedInputTokens - prev.CachedInputTokens
	outDelta := cur.OutputTokens - prev.OutputTokens
	reasonDelta := cur.ReasoningOutputTokens - prev.ReasoningOutputTokens
	if inDelta < 0 {
		inDelta = 0
	}
	if cachedDelta < 0 {
		cachedDelta = 0
	}
	if outDelta < 0 {
		outDelta = 0
	}
	if reasonDelta < 0 {
		reasonDelta = 0
	}
	miss := inDelta - cachedDelta
	if miss < 0 {
		miss = 0
	}
	*seq++
	emit(event.UsageEvent{
		Source:     "codex",
		Vendor:     vendor.Lookup(model, ""),
		SourceRoot: root.Path,
		RequestID:  fmt.Sprintf("%s:%d", path, *seq),
		SessionID:  strings.TrimSuffix(filepath.Base(path), ".jsonl"),
		Model:      model,
		Timestamp:  ts,
		Miss:       miss,
		CacheRead:  cachedDelta,
		Output:     outDelta + reasonDelta,
		Reasoning:  reasonDelta,
		Quality:    event.QualityAuthoritative,
		Derivation: event.DeriveDerived,
	})
}
