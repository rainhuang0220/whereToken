package codex

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestParseCumulativeDeltas(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "codex")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "codex", Path: root}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 80 || evs[0].CacheRead != 20 || evs[0].Output != 35 {
		t.Fatalf("first %+v", evs[0])
	}
	if evs[1].Miss != 30 || evs[1].CacheRead != 20 || evs[1].Output != 13 {
		t.Fatalf("second %+v", evs[1])
	}
	if evs[0].Vendor != "openai" || evs[0].Source != "codex" {
		t.Fatalf("axes %+v", evs[0])
	}
	sum := metric.Aggregate(evs, turns)
	if sum.All.Requests != 2 {
		t.Fatalf("requests=%d", sum.All.Requests)
	}
	if sum.All.UserTurns != 1 {
		t.Fatalf("turns=%d", sum.All.UserTurns)
	}
}

func TestLastTokenUsageFallback(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "sessions", "2026", "01", "01", "rollout-last.jsonl")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":"2026-01-01T00:00:00Z","type":"turn_context","payload":{"model":"gpt-5"}}
{"timestamp":"2026-01-01T00:00:01Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5}}}}
{"timestamp":"2026-01-01T00:00:02Z","type":"event_msg","payload":{"type":"token_count","info":{"last_token_usage":{"input_tokens":100,"cached_input_tokens":20,"output_tokens":30,"reasoning_output_tokens":5}}}}
`
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "codex", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 80 || evs[0].CacheRead != 20 || evs[0].Output != 35 {
		t.Fatalf("%+v", evs[0])
	}
}
