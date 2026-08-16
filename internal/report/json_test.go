package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func goldenText(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return strings.ReplaceAll(string(b), "\r\n", "\n")
}

func TestGoldenTextNormalizesCRLF(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "x.txt")
	if err := os.WriteFile(p, []byte("hello\r\nworld\r\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := goldenText(t, p); got != "hello\nworld\n" {
		t.Fatalf("%q", got)
	}
}

func TestWriteJSONHasP0AndNoClock(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	raw := buf.String()
	if strings.Contains(raw, "scanned_at") || strings.Contains(raw, "T15:00") {
		t.Fatalf("clock leaked:\n%s", raw)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["total_m"] != "11.68 M" {
		t.Fatalf("total_m=%v", m["total_m"])
	}
	if m["schema"].(float64) != 1 {
		t.Fatalf("schema=%v", m["schema"])
	}
	if m["hit_rate_text"] != "85.2%" {
		t.Fatalf("hit=%v", m["hit_rate_text"])
	}
	if _, ok := m["last_7d"]; !ok {
		t.Fatal("missing last_7d")
	}
	if last, ok := m["last_7d"].([]any); !ok || len(last) != 7 {
		t.Fatalf("last_7d must be 7 local days, got %#v", m["last_7d"])
	}
	tools := m["tools"].([]any)
	row := tools[0].(map[string]any)
	if row["share"] != "91.2%" {
		t.Fatalf("share=%v", row["share"])
	}
	for _, k := range []string{"period", "total", "total_m", "hit_rate_text", "requests", "user_turns", "tools", "vendors", "notes"} {
		if _, ok := m[k]; !ok {
			t.Fatalf("missing key %s", k)
		}
	}
	if m["requests"].(float64) != 3 {
		t.Fatalf("requests=%v", m["requests"])
	}
}

func TestWriteJSONGolden(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join("testdata", "default.json")
	if os.Getenv("UPDATE_GOLDEN") == "1" {
		if err := os.WriteFile(path, buf.Bytes(), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want := goldenText(t, path)
	if want != buf.String() {
		t.Fatalf("json golden mismatch\n--- got ---\n%s\n--- want ---\n%s", buf.String(), want)
	}
}

func TestGoldenTables(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	cases := []struct {
		name string
		fil  Filter
		opt  Options
		ev   bool
	}{
		{name: "default", ev: true},
		{name: "zero"},
		{name: "ascii", opt: Options{ASCII: true}},
		{name: "today", fil: Filter{Today: true}, ev: true},
		{name: "claude", fil: Filter{Tool: "claude"}, ev: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			var events []event.UsageEvent
			var turns []event.TurnEvent
			if c.ev {
				events, turns = fixture(loc)
			}
			snap, err := Build(events, turns, nil, c.fil, now, loc)
			if err != nil {
				t.Fatal(err)
			}
			got := Render(snap, c.opt)
			path := filepath.Join("testdata", c.name+".txt")
			if os.Getenv("UPDATE_GOLDEN") == "1" {
				if err := os.MkdirAll("testdata", 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			want := goldenText(t, path)
			if want != got {
				t.Fatalf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", c.name, got, want)
			}
		})
	}
}

func TestWriteJSONTodayOmitsLast7AndStreaks(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Today: true}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["last_7d"]; ok {
		t.Fatalf("today json should omit last_7d: %s", buf.String())
	}
	if _, ok := m["max_streak_days"]; ok {
		t.Fatalf("today json should omit streaks: %s", buf.String())
	}
	if _, ok := m["models"]; !ok {
		t.Fatal("today json should include models")
	}
	if m["schema"].(float64) != 1 {
		t.Fatalf("schema=%v", m["schema"])
	}
}

func TestWriteJSONModelSetsHideTurns(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Model: "k3"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	if m["hide_turns"] != true {
		t.Fatalf("hide_turns=%v json=%s", m["hide_turns"], buf.String())
	}
	if m["user_turns"].(float64) != 0 {
		t.Fatalf("user_turns=%v", m["user_turns"])
	}
}

func TestWriteJSONRowIncludesRawTotal(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	tools := m["tools"].([]any)
	row := tools[0].(map[string]any)
	total, ok := row["total"].(float64)
	if !ok {
		t.Fatalf("tools[0].total missing: %s", buf.String())
	}
	if total != 10_650_000 {
		t.Fatalf("tools[0].total=%v", total)
	}
	vendors := m["vendors"].([]any)
	v0 := vendors[0].(map[string]any)
	if v0["total"].(float64) != 10_100_000 {
		t.Fatalf("vendors[0].total=%v", v0["total"])
	}
}

func TestWriteJSONSatisfiesPublishedSchema(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "cli-json.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema struct {
		Required   []string       `json:"required"`
		Properties map[string]any `json:"properties"`
		Defs       struct {
			Row struct {
				Required []string `json:"required"`
			} `json:"row"`
		} `json:"$defs"`
	}
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	if len(schema.Required) < 8 {
		t.Fatalf("schema required too small: %v", schema.Required)
	}
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	for _, k := range schema.Required {
		if _, ok := m[k]; !ok {
			t.Errorf("missing required %q", k)
		}
	}
	for k := range m {
		if _, ok := schema.Properties[k]; !ok {
			t.Errorf("undeclared key %q", k)
		}
	}
	tools := m["tools"].([]any)
	row := tools[0].(map[string]any)
	for _, k := range schema.Defs.Row.Required {
		if _, ok := row[k]; !ok {
			t.Errorf("tools row missing %q", k)
		}
	}
}
