package report

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/community"
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
	for _, k := range []string{"period", "total", "total_m", "hit_rate_text", "requests", "user_turns", "tools", "vendors", "notes", "max_streak_days", "current_streak_days"} {
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

func TestWriteJSONAllTimeKeepsZeroStreaks(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
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
	max, ok := m["max_streak_days"]
	if !ok {
		t.Fatalf("all-time --json must keep max_streak_days even when 0: %s", buf.String())
	}
	cur, ok := m["current_streak_days"]
	if !ok {
		t.Fatalf("all-time --json must keep current_streak_days even when 0: %s", buf.String())
	}
	if max.(float64) != 0 || cur.(float64) != 0 {
		t.Fatalf("empty all-time streaks max=%v current=%v", max, cur)
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
	schema := loadPublishedCLIJSONSchema(t)
	if len(schema.Required) < 8 {
		t.Fatalf("schema required too small: %v", schema.Required)
	}
	if len(schema.Defs.Community.Properties) == 0 {
		t.Fatal("schema $defs/community has no properties")
	}
	if schema.Defs.Standing.Properties["rank"] == nil {
		t.Fatal("schema $defs/standing must declare rank")
	}

	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	base, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}

	unconfigured := community.EmptyView(community.StatusServiceUnconfigured, "Community Rank service is not configured.")
	zeroPodium := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	zeroPodium.Today.Rank = 0
	zeroPodium.Today.Display = "#0 / 20"
	zeroPodium.All = zeroPodium.Today
	zeroPodium.All.Period = community.PeriodAll

	ranked := community.EmptyView(community.StatusOK, community.DisclaimerEN)
	ranked.Today = community.FinishStanding(community.StatusOK, community.PeriodToday, community.MetricTokens, 37, 842, 20)
	ranked.All = community.FinishStanding(community.StatusOK, community.PeriodAll, community.MetricTokens, 12, 842, 20)

	cases := []struct {
		name string
		comm *community.View
		rank float64
	}{
		{name: "without community"},
		{name: "unconfigured community", comm: &unconfigured},
		{name: "zero podium sanitized", comm: &zeroPodium},
		{name: "real rank", comm: &ranked, rank: 37},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			snap := base
			if tc.comm != nil {
				snap.Community = *tc.comm
			}
			got := writeJSONMap(t, snap)
			m := got.obj
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
			if tc.comm == nil {
				if _, ok := m["community"]; ok {
					t.Fatalf("community should be omitted: %s", got.raw)
				}
				return
			}
			assertCommunityAgainstSchema(t, m, schema)
			comm := m["community"].(map[string]any)
			today := comm["today"].(map[string]any)
			if tc.rank == 0 {
				if _, has := today["rank"]; has {
					t.Fatalf("rank must be omitted: %s", got.raw)
				}
			} else if today["rank"] != tc.rank {
				t.Fatalf("rank=%v want %v", today["rank"], tc.rank)
			}
		})
	}
}

func TestWriteJSONCommunitySanitizesZeroRank(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	snap.Community = community.EmptyView(community.StatusOK, community.DisclaimerEN)
	snap.Community.Today.Rank = 0
	snap.Community.Today.Display = "#0 / 20"
	snap.Community.All = snap.Community.Today
	snap.Community.All.Period = community.PeriodAll
	got := writeJSONMap(t, snap)
	assertCommunityAgainstSchema(t, got.obj, loadPublishedCLIJSONSchema(t))
	comm, ok := got.obj["community"].(map[string]any)
	if !ok {
		t.Fatalf("missing community: %s", got.raw)
	}
	today := comm["today"].(map[string]any)
	if _, has := today["rank"]; has {
		t.Fatalf("rank must be omitted: %s", got.raw)
	}
	if today["status"] == "ok" {
		t.Fatalf("zero podium must not stay ok: %s", got.raw)
	}
	if strings.Contains(got.raw, "#0") {
		t.Fatalf("#0 leaked: %s", got.raw)
	}
}

type publishedCLIJSONSchema struct {
	Required   []string       `json:"required"`
	Properties map[string]any `json:"properties"`
	Defs       struct {
		Row struct {
			Required []string `json:"required"`
		} `json:"row"`
		Community struct {
			Properties map[string]any `json:"properties"`
		} `json:"community"`
		Standing struct {
			Required   []string       `json:"required"`
			Properties map[string]any `json:"properties"`
		} `json:"standing"`
	} `json:"$defs"`
}

type jsonMap struct {
	raw string
	obj map[string]any
}

func writeJSONMap(t *testing.T, snap Snapshot) jsonMap {
	t.Helper()
	var buf bytes.Buffer
	if err := WriteJSON(&buf, snap); err != nil {
		t.Fatal(err)
	}
	var m map[string]any
	if err := json.Unmarshal(buf.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	return jsonMap{raw: buf.String(), obj: m}
}

func loadPublishedCLIJSONSchema(t *testing.T) publishedCLIJSONSchema {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	raw, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "cli-json.schema.json"))
	if err != nil {
		t.Fatal(err)
	}
	var schema publishedCLIJSONSchema
	if err := json.Unmarshal(raw, &schema); err != nil {
		t.Fatal(err)
	}
	return schema
}

func assertCommunityAgainstSchema(t *testing.T, payload map[string]any, schema publishedCLIJSONSchema) {
	t.Helper()
	raw, ok := payload["community"]
	if !ok {
		t.Fatal("community missing")
	}
	comm, ok := raw.(map[string]any)
	if !ok {
		t.Fatalf("community must be an object, got %T", raw)
	}
	assertDeclaredKeys(t, comm, schema.Defs.Community.Properties, "community")
	for _, period := range []string{"today", "all"} {
		stRaw, ok := comm[period]
		if !ok {
			continue
		}
		st, ok := stRaw.(map[string]any)
		if !ok {
			t.Fatalf("community.%s must be an object, got %T", period, stRaw)
		}
		path := "community." + period
		assertDeclaredKeys(t, st, schema.Defs.Standing.Properties, path)
		for _, req := range schema.Defs.Standing.Required {
			if _, ok := st[req]; !ok {
				t.Errorf("%s missing required %q", path, req)
			}
		}
		if rank, has := st["rank"]; has {
			n, ok := jsonNumber(rank)
			min := standingMinimum(schema, "rank")
			if min < 1 {
				min = 1
			}
			if !ok || n < min || n != float64(int64(n)) {
				t.Errorf("%s.rank=%v must be an integer >= %v (never 0)", path, rank, min)
			}
		}
	}
}

func assertDeclaredKeys(t *testing.T, obj map[string]any, allowed map[string]any, path string) {
	t.Helper()
	for k := range obj {
		if _, ok := allowed[k]; !ok {
			t.Errorf("%s undeclared key %q", path, k)
		}
	}
}

func standingMinimum(schema publishedCLIJSONSchema, field string) float64 {
	spec, ok := schema.Defs.Standing.Properties[field].(map[string]any)
	if !ok {
		return 0
	}
	min, _ := spec["minimum"].(float64)
	return min
}

func jsonNumber(v any) (float64, bool) {
	n, ok := v.(float64)
	return n, ok
}
