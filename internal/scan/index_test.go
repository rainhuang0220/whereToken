package scan

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/claude"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestScanSecondPassIsUnchanged(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	t.Setenv("WHERETOKEN_NO_INDEX", "")
	dir := t.TempDir()
	proj := filepath.Join(dir, ".claude", "projects", "x")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	var b strings.Builder
	for i := 0; i < 200; i++ {
		b.WriteString(`{"type":"assistant","requestId":"r` + itoa(i) + `","message":{"model":"claude-opus-4.6","usage":{"input_tokens":2,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n")
	}
	if err := os.WriteFile(filepath.Join(proj, "s.jsonl"), []byte(b.String()), 0o644); err != nil {
		t.Fatal(err)
	}
	home := testhome.New(dir)
	ads := []adapter.Adapter{claude.Adapter{}}
	first := Run(home, ads)
	if first.Summary.All.Requests != 200 {
		t.Fatalf("first requests=%d", first.Summary.All.Requests)
	}
	if !hasMode(first.Deltas, "claude", index.ModeFull) {
		t.Fatalf("first deltas=%+v", first.Deltas)
	}
	second := Run(home, ads)
	if second.Summary.All.Requests != 200 || second.Summary.All.Total() != first.Summary.All.Total() {
		t.Fatalf("second %+v vs first %+v", second.Summary.All, first.Summary.All)
	}
	if !hasMode(second.Deltas, "claude", index.ModeUnchanged) {
		t.Fatalf("second should be unchanged: %+v", second.Deltas)
	}
}

func TestScanAppendIsIncremental(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	proj := filepath.Join(dir, ".claude", "projects", "x")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, "s.jsonl")
	line := func(id string, miss int) string {
		return `{"type":"assistant","requestId":"` + id + `","message":{"model":"claude-opus-4.6","usage":{"input_tokens":` + itoa(miss) + `,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	}
	if err := os.WriteFile(path, []byte(line("a", 10)), 0o644); err != nil {
		t.Fatal(err)
	}
	home := testhome.New(dir)
	ads := []adapter.Adapter{claude.Adapter{}}
	first := Run(home, ads)
	if first.Summary.All.Miss != 10 {
		t.Fatalf("first miss=%d", first.Summary.All.Miss)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(line("b", 7)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	_ = os.Chtimes(path, time.Now().Add(2*time.Second), time.Now().Add(2*time.Second))
	second := Run(home, ads)
	if second.Summary.All.Miss != 17 || second.Summary.All.Requests != 2 {
		t.Fatalf("after append %+v", second.Summary.All)
	}
	if !hasMode(second.Deltas, "claude", index.ModeIncremental) {
		t.Fatalf("want incremental %+v", second.Deltas)
	}
}

func TestScanTruncateIsFull(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	proj := filepath.Join(dir, ".claude", "projects", "x")
	if err := os.MkdirAll(proj, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(proj, "s.jsonl")
	long := `{"type":"assistant","requestId":"old","message":{"model":"claude-opus-4.6","usage":{"input_tokens":99,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	if err := os.WriteFile(path, []byte(long+long), 0o644); err != nil {
		t.Fatal(err)
	}
	home := testhome.New(dir)
	ads := []adapter.Adapter{claude.Adapter{}}
	_ = Run(home, ads)
	short := `{"type":"assistant","requestId":"new","message":{"model":"claude-opus-4.6","usage":{"input_tokens":3,"output_tokens":1,"cache_read_input_tokens":0,"cache_creation_input_tokens":0}}}` + "\n"
	if err := os.WriteFile(path, []byte(short), 0o644); err != nil {
		t.Fatal(err)
	}
	second := Run(home, ads)
	if second.Summary.All.Miss != 3 || second.Summary.All.Requests != 1 {
		t.Fatalf("truncate must rescan: %+v", second.Summary.All)
	}
	if !hasMode(second.Deltas, "claude", index.ModeFull) {
		t.Fatalf("truncate deltas=%+v", second.Deltas)
	}
}

func TestFormatDeltas(t *testing.T) {
	s := FormatDeltas([]index.Delta{
		{Source: "claude", Mode: index.ModeIncremental, Added: 124},
		{Source: "kimi", Mode: index.ModeUnchanged},
	})
	if !strings.Contains(s, "Scanning usage data") || !strings.Contains(s, "Claude Code") || !strings.Contains(s, "incremental") || !strings.Contains(s, "+124") || !strings.Contains(s, "unchanged") {
		t.Fatalf("%s", s)
	}
}

func TestApplyWindowFiltersEvents(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, loc)
	r := Result{
		Events: []event.UsageEvent{
			{Source: "kimi", RequestID: "old", Miss: 10, Timestamp: now.AddDate(0, 0, -10)},
			{Source: "kimi", RequestID: "new", Miss: 4, Timestamp: now.Add(-time.Hour)},
		},
	}
	win, err := metric.ParseWindow(false, "7d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	got := ApplyWindow(r, win, loc)
	if got.Summary.All.Miss != 4 || got.Summary.All.Requests != 1 {
		t.Fatalf("window %+v", got.Summary.All)
	}
}

func hasMode(ds []index.Delta, source, mode string) bool {
	for _, d := range ds {
		if d.Source == source && d.Mode == mode {
			return true
		}
	}
	return false
}
