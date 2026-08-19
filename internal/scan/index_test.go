package scan

import (
	"encoding/json"
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

func TestCompareWindowsMatrix(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, loc)
	r := Result{
		Events: []event.UsageEvent{
			{Source: "kimi", RequestID: "old", Miss: 10, Timestamp: now.AddDate(0, 0, -20)},
			{Source: "kimi", RequestID: "new", Miss: 4, Timestamp: now.Add(-time.Hour)},
		},
	}
	must := func(w metric.Window, err error) metric.Window {
		t.Helper()
		if err != nil {
			t.Fatal(err)
		}
		return w
	}
	cases := []struct {
		name    string
		win     metric.Window
		compare bool
	}{
		{"all", must(metric.ParseWindow(false, "", "", "", now, loc)), false},
		{"today", must(metric.ParseWindow(true, "", "", "", now, loc)), true},
		{"since-7d", must(metric.ParseWindow(false, "7d", "", "", now, loc)), true},
		{"since-30d", must(metric.ParseWindow(false, "30d", "", "", now, loc)), true},
		{"from-only", must(metric.ParseWindow(false, "", "2026-08-01", "", now, loc)), false},
		{"to-only", must(metric.ParseWindow(false, "", "", "2026-08-19", now, loc)), false},
		{"from-to", must(metric.ParseWindow(false, "", "2026-08-01", "2026-08-19", now, loc)), true},
	}
	for _, c := range cases {
		got := CompareWindows(r, c.win, loc)
		if c.compare && got == nil {
			t.Fatalf("%s: want compare", c.name)
		}
		if !c.compare && got != nil {
			t.Fatalf("%s: compare must be nil", c.name)
		}
	}
}

func TestCompareWindowsOmitsUnavailableCostUSD(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 8, 19, 12, 0, 0, 0, loc)
	r := Result{
		Events: []event.UsageEvent{
			{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "old", Miss: 1_000_000, Output: 1_000_000, Timestamp: now.AddDate(0, 0, -20)},
			{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "new", Miss: 100, Output: 10, Timestamp: now.Add(-time.Hour)},
		},
	}
	win, err := metric.ParseWindow(false, "7d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	cur := ApplyWindow(r, win, loc)
	cmp := CompareWindows(r, win, loc)
	if cmp == nil {
		t.Fatal("want compare")
	}
	cur.Compare = cmp
	if cur.Summary.All.CostStatus != "unavailable" || cur.Summary.All.CostMicro != 0 || cur.Summary.All.Total() != 110 {
		t.Fatalf("windowed all %+v", cur.Summary.All)
	}
	v := metric.View(cur.Summary.All)
	if v.CostUSD != "" || v.CostStatus != "unavailable" {
		t.Fatalf("must omit $0: %+v", v)
	}

	raw, err := MarshalSummary(cur)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "$0") {
		t.Fatalf("never $0:\n%s", raw)
	}
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		t.Fatal(err)
	}
	all, _ := payload["all"].(map[string]any)
	if all["cost_status"] != "unavailable" {
		t.Fatalf("status=%v", all["cost_status"])
	}
	if _, ok := all["cost_usd"]; ok {
		t.Fatalf("windowed summary must omit cost_usd: %v", all)
	}
	assertJSONOmitsCostUSD(t, payload["by_source"])
	assertJSONOmitsCostUSD(t, payload["by_vendor"])
	assertJSONOmitsCostUSD(t, payload["by_source_vendor"])
	drill, _ := payload["drill"].(map[string]any)
	drillAll, _ := drill["all"].(map[string]any)
	assertJSONOmitsCostUSD(t, drillAll["models"])
	assertJSONOmitsCostUSD(t, drillAll["sessions"])
	assertJSONOmitsCostUSD(t, drillAll["workspaces"])

	full := r
	full.Summary = metric.Aggregate(r.Events, nil)
	fullRaw, err := MarshalSummary(full)
	if err != nil {
		t.Fatal(err)
	}
	var fullPayload map[string]any
	if err := json.Unmarshal(fullRaw, &fullPayload); err != nil {
		t.Fatal(err)
	}
	fullAll, _ := fullPayload["all"].(map[string]any)
	if fullAll["cost_usd"] != "$30.0000" || fullAll["cost_status"] != "partial" {
		t.Fatalf("all-time priced mix %+v", fullAll)
	}
}

func assertJSONOmitsCostUSD(t *testing.T, rows any) {
	t.Helper()
	list, ok := rows.([]any)
	if !ok {
		t.Fatalf("rows=%T", rows)
	}
	for _, item := range list {
		row, _ := item.(map[string]any)
		if _, ok := row["cost_usd"]; ok {
			t.Fatalf("unavailable slice must omit cost_usd: %v", row)
		}
	}
}

func TestApplyWindowDoesNotMarkHistoricalSourceAbsent(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, loc)
	r := Result{
		Roots: []adapter.SourceRoot{{ID: "openclaw", Path: "/tmp/openclaw"}, {ID: "kimi", Path: "/tmp/kimi"}},
		Events: []event.UsageEvent{
			{Source: "openclaw", RequestID: "old", Miss: 10, Quality: event.QualityAuthoritative, Timestamp: now.AddDate(0, 0, -10)},
			{Source: "kimi", RequestID: "new", Miss: 4, Quality: event.QualityAuthoritative, Timestamp: now.Add(-time.Hour)},
		},
	}
	win, err := metric.ParseWindow(true, "", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	got := ApplyWindow(r, win, loc)
	if got.Summary.All.Miss != 4 {
		t.Fatalf("today %+v", got.Summary.All)
	}
	for _, s := range got.Summary.BySource {
		if s.ID == "openclaw" && s.Quality == event.QualityAbsent {
			t.Fatalf("history outside the window is not missing data: %+v", s)
		}
		if s.ID == "openclaw" && s.Total() != 0 {
			t.Fatalf("today must not import last week's openclaw: %+v", s)
		}
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
