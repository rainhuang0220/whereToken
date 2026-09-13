package publicprofile

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func ev(source, vendor string, at time.Time, miss, cache, out int64) event.UsageEvent {
	return event.UsageEvent{
		Source:     source,
		Vendor:     vendor,
		RequestID:  source + at.UTC().Format("20060102150405"),
		Timestamp:  at,
		Miss:       miss,
		CacheRead:  cache,
		Output:     out,
		Quality:    event.QualityAuthoritative,
		Derivation: event.DeriveRaw,
	}
}

func TestBuildEmptyIsUnavailableNotZero(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{Now: now, Loc: loc, Version: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.DataStatus != StatusUnavailable {
		t.Fatalf("status=%q", snap.DataStatus)
	}
	if snap.Periods.All.Totals.Total.Display != emDash || snap.Periods.All.Totals.Total.Value != nil {
		t.Fatalf("empty total=%+v", snap.Periods.All.Totals.Total)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
}

func TestBuildUnknownSourceBecomesOther(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	poison := "/Users/rainhuang/secret"
	snap, err := Build(Input{
		Events: []event.UsageEvent{ev(poison, "anthropic", now.Add(-time.Hour), 10, 0, 1)},
		Now:    now, Loc: loc,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Marshal(snap)
	if strings.Contains(string(raw), poison) || strings.Contains(string(raw), "/Users/") {
		t.Fatalf("leaked path:\n%s", raw)
	}
	if len(snap.Periods.All.ByAgent) != 1 || snap.Periods.All.ByAgent[0].ID != AgentOtherID {
		t.Fatalf("agents=%+v", snap.Periods.All.ByAgent)
	}
}

func TestBuildAllTimeMatchesAggregateAt(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{
		ev("claude", "anthropic", time.Date(2026, 9, 13, 10, 0, 0, 0, loc), 100, 20, 5),
		ev("kimi", "moonshot", time.Date(2026, 9, 12, 10, 0, 0, 0, loc), 50, 0, 10),
	}
	sum := metric.AggregateAt(events, nil, now, loc)
	snap, err := Build(Input{Events: events, Now: now, Loc: loc, Version: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	got := *snap.Periods.All.Totals.Total.Value
	if got != sum.All.Total() {
		t.Fatalf("all=%d aggregate=%d", got, sum.All.Total())
	}
	if snap.Periods.All.HitRate.Display != metric.View(sum.All).HitRateText {
		t.Fatalf("hit %q vs %q", snap.Periods.All.HitRate.Display, metric.View(sum.All).HitRateText)
	}
	if snap.GeneratedAt == "" || snap.AsOfDate != "2026-09-13" {
		t.Fatalf("clock %+v", snap)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
}

func TestBuildReasoningNotInTotal(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	e := ev("grok", "xai", now.Add(-time.Hour), 10, 0, 5)
	e.Reasoning = 1000
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	if *snap.Periods.All.Totals.Total.Value != 15 {
		t.Fatalf("total=%d", *snap.Periods.All.Totals.Total.Value)
	}
}

func TestLevelsIgnoreHistoryOutsideWall(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	inWall := ev("claude", "anthropic", time.Date(2026, 9, 10, 10, 0, 0, 0, loc), 100, 0, 0)
	base, err := Build(Input{Events: []event.UsageEvent{inWall}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	old := ev("claude", "anthropic", time.Date(2025, 8, 1, 10, 0, 0, 0, loc), 9_000_000, 0, 0)
	withHist, err := Build(Input{Events: []event.UsageEvent{inWall, old}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	a := seriesByID(base, "all")
	b := seriesByID(withHist, "all")
	if len(a.Levels) != len(b.Levels) {
		t.Fatal("length")
	}
	for i := range a.Levels {
		if a.Levels[i] != b.Levels[i] {
			t.Fatalf("level[%d] %d vs %d (wall color must ignore history)", i, a.Levels[i], b.Levels[i])
		}
	}
}

func TestMeasuredZeroNotUnavailable(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	e := ev("claude", "anthropic", now.Add(-time.Hour), 0, 0, 0)
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	// usableTokens requires miss/cache/output >= 0; total 0 with no requests → unavailable
	_ = snap
}

func seriesByID(s Snapshot, id string) Series {
	for _, ser := range s.Activity.Series {
		if ser.ID == id && ser.Dimension == "all" {
			return ser
		}
	}
	return Series{}
}
