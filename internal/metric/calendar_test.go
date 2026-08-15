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
