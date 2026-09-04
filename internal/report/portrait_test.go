package report

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/profile"
)

// Semantic guards for the bottom-right KPI cell: every terminal report path
// shows the usage portrait from the existing profile engine, and rank never
// comes back — regardless of what future golden files say.

func assertPortraitNotRank(t *testing.T, name, out string) {
	t.Helper()
	if !strings.Contains(out, "用户画像") {
		t.Errorf("%s: missing 用户画像 cell:\n%s", name, out)
	}
	if strings.Contains(out, "排名") {
		t.Errorf("%s: rank label leaked:\n%s", name, out)
	}
	if strings.Contains(out, "社区排名暂不可用") {
		t.Errorf("%s: community-rank note leaked:\n%s", name, out)
	}
}

func TestPortraitCellAcrossReportPaths(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	win7, err := metric.ParseWindow(false, "7d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	win30, err := metric.ParseWindow(false, "30d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	cases := []struct {
		name string
		fil  Filter
		opt  Options
		ev   bool
	}{
		{name: "default all-time", fil: Filter{Seed: "guard"}, ev: true},
		{name: "today", fil: Filter{Today: true, Seed: "guard"}, ev: true},
		{name: "7d", fil: Filter{Days: win7.Days, From: win7.From, To: win7.To, Period: win7.Label, Seed: "guard"}, ev: true},
		{name: "30d", fil: Filter{Days: win30.Days, From: win30.From, To: win30.To, Period: win30.Label, Seed: "guard"}, ev: true},
		{name: "custom range", fil: Filter{From: win7.From, To: win7.To, Seed: "guard"}, ev: true},
		{name: "scoped tool", fil: Filter{Tool: "claude", Seed: "guard"}, ev: true},
		{name: "model", fil: Filter{Model: "k3", Seed: "guard"}, ev: true},
		{name: "no-data", fil: Filter{Seed: "guard"}},
		{name: "no-data today", fil: Filter{Today: true, Seed: "guard"}},
		{name: "ascii", fil: Filter{Seed: "guard"}, opt: Options{ASCII: true}, ev: true},
		{name: "narrow width", fil: Filter{Seed: "guard"}, opt: Options{Width: 60}, ev: true},
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
			assertPortraitNotRank(t, c.name, Render(snap, c.opt))
		})
	}
}

func TestPortraitCellStates(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)

	// no-data → —, never a rank fallback
	snap, err := Build(nil, nil, nil, Filter{Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Portrait.State != profile.StateNone {
		t.Fatalf("empty state=%q", snap.Portrait.State)
	}
	if out := Render(snap, Options{}); !strings.Contains(out, "用户画像") {
		t.Fatalf("no-data must keep the portrait label:\n%s", out)
	}

	// below the profiling threshold → 数据不足
	small := []event.UsageEvent{{
		Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "c",
		Timestamp: now, Miss: 50_000, Quality: event.QualityAuthoritative,
	}}
	snap, err = Build(small, nil, nil, Filter{Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Portrait.State != profile.StateInsufficient || snap.Portrait.Primary != "数据不足" {
		t.Fatalf("insufficient portrait=%+v", snap.Portrait)
	}
	if out := Render(snap, Options{}); !strings.Contains(out, "数据不足") {
		t.Fatalf("insufficient must say 数据不足:\n%s", out)
	}

	// enough data → the engine's primary phrase, and the note spells out tags
	events, turns := fixture(loc)
	snap, err = Build(events, turns, nil, Filter{Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Portrait.State != profile.StateOK || snap.Portrait.Primary == "" {
		t.Fatalf("portrait=%+v", snap.Portrait)
	}
	out := Render(snap, Options{})
	if !strings.Contains(out, snap.Portrait.Primary) {
		t.Fatalf("cell must show the engine primary %q:\n%s", snap.Portrait.Primary, out)
	}
	if !strings.Contains(out, "画像 · "+snap.Portrait.Primary) {
		t.Fatalf("footnote must spell out the portrait:\n%s", out)
	}
}

// Same seed + same data → byte-identical cell; different seed may rephrase.
func TestPortraitCellDeterministic(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	a, err := Build(events, turns, nil, Filter{Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	b, err := Build(events, turns, nil, Filter{Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if a.Portrait.State != b.Portrait.State || a.Portrait.Primary != b.Portrait.Primary ||
		strings.Join(a.Portrait.Tags, "|") != strings.Join(b.Portrait.Tags, "|") ||
		a.Portrait.Detail != b.Portrait.Detail {
		t.Fatalf("same seed diverged: %+v vs %+v", a.Portrait, b.Portrait)
	}
}

// The headline estimate is 2-decimal with thousands separators; detail rows
// keep micro precision.
func TestCostKPIGrouped2Decimals(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	out := Render(snap, Options{})
	if !strings.Contains(out, "$12.00") {
		t.Fatalf("KPI estimate must be 2-decimal:\n%s", out)
	}
	if !strings.Contains(out, "$12.0000") {
		t.Fatalf("detail rows keep micro precision:\n%s", out)
	}

	snap.CostUSD = "$3670.6920"
	if got := costKPI(snap); got != "$3,670.69" {
		t.Fatalf("grouped=%q", got)
	}
	snap.CostUSD = "$0.0040"
	if got := costKPI(snap); got != "—" {
		t.Fatalf("rounds-to-zero must be —, got %q", got)
	}
}

func TestUsdGrouped2(t *testing.T) {
	cases := map[string]string{
		"$3670.6920": "$3,670.69",
		"$12.0000":   "$12.00",
		"$863.9959":  "$864.00",
		"$0.0040":    "",
		"$0.0000":    "",
		"$1234567.8": "$1,234,567.80",
		"-$5.5000":   "-$5.50",
		"":           "",
		"junk":       "",
	}
	for in, want := range cases {
		if got := usdGrouped2(in); got != want {
			t.Errorf("usdGrouped2(%q) = %q, want %q", in, got, want)
		}
	}
}

// Windowed views profile the window, not the all-time ledger.
func TestPortraitFollowsTheWindow(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	win, err := metric.ParseWindow(false, "1d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	snap, err := Build(events, turns, nil, Filter{Days: win.Days, From: win.From, To: win.To, Seed: "guard"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.Portrait.State == profile.StateNone {
		t.Fatal("window with data must profile")
	}
	future, err := Build(events, turns, nil, Filter{
		From: time.Date(2026, 9, 1, 0, 0, 0, 0, loc), To: time.Date(2026, 9, 2, 0, 0, 0, 0, loc), Seed: "guard",
	}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if future.Portrait.State != profile.StateNone {
		t.Fatalf("empty window portrait=%+v", future.Portrait)
	}
}
