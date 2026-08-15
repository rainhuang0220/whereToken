package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

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
	if m["hit_rate_text"] != "85.2%" {
		t.Fatalf("hit=%v", m["hit_rate_text"])
	}
	if m["requests"].(float64) != 3 {
		t.Fatalf("requests=%v", m["requests"])
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
			want, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("read golden %s: %v\n--- got ---\n%s", path, err, got)
			}
			if string(want) != got {
				t.Fatalf("golden %s mismatch\n--- got ---\n%s\n--- want ---\n%s", c.name, got, want)
			}
		})
	}
}
