package metric

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func ts(loc *time.Location, y, m, d, hh, mm int) time.Time {
	return time.Date(y, time.Month(m), d, hh, mm, 0, 0, loc)
}

func TestBuildCalendarMergesSameLocalDay(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 15, 1, 0), Miss: 10, CacheRead: 90, Output: 5},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 23, 0), Miss: 20, Output: 3},
	}
	cal := BuildCalendar(events, loc, now)
	if len(cal.All.Days) != 1 {
		t.Fatalf("days=%d", len(cal.All.Days))
	}
	d := cal.All.Days[0]
	if d.Date != "2026-08-15" {
		t.Fatalf("date=%s", d.Date)
	}
	if d.Total != 10+90+5+20+3 {
		t.Fatalf("total=%d", d.Total)
	}
	if d.Miss != 30 || d.CacheRead != 90 || d.Output != 8 {
		t.Fatalf("fields %+v", d)
	}
}

func TestBuildCalendarConservationMatchesAggregate(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 100, CacheRead: 900, Output: 10},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 50, Output: 5},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Timestamp: ts(loc, 2026, 8, 15, 11, 0), Miss: 20, CacheRead: 80, Output: 3},
	}
	sum := Aggregate(events, nil)
	cal := BuildCalendar(events, loc, now)
	var daySum int64
	for _, d := range cal.All.Days {
		daySum += d.Total
	}
	if daySum != sum.All.Total() {
		t.Fatalf("days=%d all=%d", daySum, sum.All.Total())
	}
}

func TestBuildCalendarEmptyDayBreaksStreak(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "1", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 1},
		{Source: "kimi", Vendor: "moonshot", RequestID: "2", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 1},
		{Source: "kimi", Vendor: "moonshot", RequestID: "4", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 1},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.LongestStreak != 2 {
		t.Fatalf("longest=%d", cal.All.Stats.LongestStreak)
	}
	if cal.All.Stats.CurrentStreak != 1 {
		t.Fatalf("current=%d want 1 (yesterday only; today empty)", cal.All.Stats.CurrentStreak)
	}
}

func TestBuildCalendarPeakPicksMaxTotalDay(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 10, 10, 0), Miss: 10},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 50},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 20},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.PeakDate != "2026-08-11" || cal.All.Stats.PeakTotal != 50 {
		t.Fatalf("peak=%s %d", cal.All.Stats.PeakDate, cal.All.Stats.PeakTotal)
	}
}

func TestBuildCalendarCurrentStreakIncludesTodayWhenUsed(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 1},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 1},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.CurrentStreak != 2 {
		t.Fatalf("current=%d", cal.All.Stats.CurrentStreak)
	}
}

func TestBuildCalendarVendorMinimaxUsesOnlyThoseEvents(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 100},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 11, 0), Miss: 40},
	}
	cal := BuildCalendar(events, loc, now)
	mm := cal.ByVendor["minimax"]
	if len(mm.Days) != 1 || mm.Days[0].Total != 40 {
		t.Fatalf("minimax days=%v", mm.Days)
	}
	if mm.Stats.PeakTotal != 40 {
		t.Fatalf("minimax peak=%d", mm.Stats.PeakTotal)
	}
	var all int64
	for _, d := range cal.All.Days {
		all += d.Total
	}
	if all != 140 {
		t.Fatalf("all=%d", all)
	}
}

func TestBuildCalendarLevelsUseSeriesQuartilesNotGlobalMax(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "big", Timestamp: ts(loc, 2026, 8, 10, 10, 0), Miss: 1_000_000},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s1", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 10},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s2", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 10},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s3", Timestamp: ts(loc, 2026, 8, 13, 10, 0), Miss: 10},
	}
	cal := BuildCalendar(events, loc, now)
	oc := cal.BySource["opencode"]
	if len(oc.Days) != 3 {
		t.Fatalf("opencode days=%d", len(oc.Days))
	}
	for _, d := range oc.Days {
		if d.Level != 2 {
			t.Fatalf("equal small days should be mid level, got %d on %s", d.Level, d.Date)
		}
	}
}

func TestLastNDailyTotalsFillsZeros(t *testing.T) {
	loc := shanghai()
	today := ts(loc, 2026, 8, 16, 12, 0)
	days := []Day{
		{Date: "2026-08-15", Total: 100},
		{Date: "2026-08-16", Total: 40},
	}
	got := LastNDailyTotals(days, today, 7)
	if len(got) != 7 {
		t.Fatalf("len=%d", len(got))
	}
	if got[5] != 100 || got[6] != 40 {
		t.Fatalf("%v", got)
	}
	for i := 0; i < 5; i++ {
		if got[i] != 0 {
			t.Fatalf("day %d = %d", i, got[i])
		}
	}
}
