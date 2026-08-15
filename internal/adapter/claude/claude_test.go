package claude

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestParseClaudeJSONL(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "claude")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "claude", Path: root}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	sum := metric.Aggregate(evs, turns)
	if sum.All.Requests != 2 {
		t.Fatalf("requests=%d events=%d", sum.All.Requests, len(evs))
	}
	if sum.All.Miss != 28 || sum.All.Output != 6 {
		t.Fatalf("all %+v", sum.All)
	}
	if sum.All.UserTurns != 1 {
		t.Fatalf("turns=%d raw=%d", sum.All.UserTurns, len(turns))
	}
	found := false
	for _, e := range evs {
		if e.RequestID != "r2" {
			continue
		}
		found = true
		if e.Vendor != "minimax" || e.Source != "claude" {
			t.Fatalf("r2 axes %+v", e)
		}
		if e.Quality != event.QualityDegraded {
			t.Fatalf("quality=%s", e.Quality)
		}
	}
	if !found {
		t.Fatal("missing MiniMax event")
	}
	for _, e := range evs {
		if e.Workspace != "-tmp-demo" || e.SessionID != "s" {
			t.Fatalf("context %+v", e)
		}
	}
}

func TestNeverReadsSettingsJSON(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "settings.json"), []byte(`{"type":"assistant","requestId":"secret","message":{"model":"claude-opus-4.6","usage":{"input_tokens":999,"output_tokens":999,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	proj := filepath.Join(dir, "projects", "x")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte(`{"type":"assistant","requestId":"r","message":{"model":"claude-opus-4.6","usage":{"input_tokens":1,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "claude", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.RequestID == "secret" || e.Miss == 999 {
			t.Fatal("parser read settings.json")
		}
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
}
