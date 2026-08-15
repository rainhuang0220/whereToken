package report

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

func ts(loc *time.Location, y, m, d, hh int) time.Time {
	return time.Date(y, time.Month(m), d, hh, 0, 0, 0, loc)
}

func fixture(loc *time.Location) ([]event.UsageEvent, []event.TurnEvent) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Timestamp: ts(loc, 2026, 8, 15, 10), Miss: 1_000_000, CacheRead: 9_000_000, Output: 100_000, Quality: event.QualityAuthoritative},
		{Source: "claude", Vendor: "minimax", Model: "MiniMax-M3", RequestID: "b", Timestamp: ts(loc, 2026, 8, 16, 11), Miss: 500_000, Output: 50_000, Quality: event.QualityAuthoritative},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "c", Timestamp: ts(loc, 2026, 8, 16, 12), Miss: 200_000, CacheRead: 800_000, Output: 30_000, Quality: event.QualityAuthoritative},
	}
	turns := []event.TurnEvent{
		{Source: "claude", Timestamp: ts(loc, 2026, 8, 15, 10)},
		{Source: "claude", Timestamp: ts(loc, 2026, 8, 16, 11)},
		{Source: "kimi", Timestamp: ts(loc, 2026, 8, 16, 12)},
	}
	return events, turns
}

func TestSnapshotP0SixSinceRecordsBegan(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "11.68 M" {
		t.Fatalf("total=%q", snap.TotalM)
	}
	if snap.HitRateText != "85.2%" {
		t.Fatalf("hit=%q", snap.HitRateText)
	}
	if snap.MaxStreak != 2 || snap.CurrentStreak != 2 {
		t.Fatalf("streak max=%d current=%d", snap.MaxStreak, snap.CurrentStreak)
	}
	if snap.Requests != 3 || snap.UserTurns != 3 {
		t.Fatalf("req=%d turns=%d", snap.Requests, snap.UserTurns)
	}
	if !snap.ShowStreaks {
		t.Fatal("default must show streaks")
	}
	if snap.Period != "有账本以来" {
		t.Fatalf("period=%q", snap.Period)
	}
}

func TestSnapshotZeroDataEmDash(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, nil, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "0.00 M" || snap.HitRateText != "—" {
		t.Fatalf("zero %+v", snap)
	}
	if snap.Requests != 0 || snap.MaxStreak != 0 {
		t.Fatalf("zero counts %+v", snap)
	}
}

func TestFilterTodayUsesLocalDate(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Today: true}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "1.58 M" {
		t.Fatalf("today total=%q", snap.TotalM)
	}
	if snap.Requests != 2 || snap.UserTurns != 2 {
		t.Fatalf("req=%d turns=%d", snap.Requests, snap.UserTurns)
	}
	if snap.ShowStreaks {
		t.Fatal("today view hides all-time streaks")
	}
	if snap.Period != "今天 2026-08-16" {
		t.Fatalf("period=%q", snap.Period)
	}
	if len(snap.Tools) < 2 {
		t.Fatalf("today tools=%v", snap.Tools)
	}
}

func TestFilterToolCursorKnownEmpty(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Tool: "cursor"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "0.00 M" || snap.Requests != 0 {
		t.Fatalf("%+v", snap)
	}
	if snap.Scope != "Cursor" {
		t.Fatalf("scope=%q", snap.Scope)
	}
}

func TestFilterToolClaude(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Tool: "claude"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "10.65 M" {
		t.Fatalf("total=%q", snap.TotalM)
	}
	if snap.UserTurns != 2 || snap.Requests != 2 {
		t.Fatalf("req=%d turns=%d", snap.Requests, snap.UserTurns)
	}
}

func TestFilterTodayAndTool(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Today: true, Tool: "claude"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "0.55 M" || snap.Requests != 1 || snap.UserTurns != 1 {
		t.Fatalf("%+v", snap)
	}
}

func TestUnknownModelIsUsage(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	_, err := Build(events, turns, nil, Filter{Model: "nope-model"}, now, loc)
	if err == nil || !isUsage(err) {
		t.Fatalf("err=%v", err)
	}
	if !strings.Contains(err.Error(), "nope-model") {
		t.Fatalf("err=%v", err)
	}
}

func TestFilterModelK3(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	events, turns := fixture(loc)
	snap, err := Build(events, turns, nil, Filter{Model: "k3"}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if snap.TotalM != "1.03 M" || snap.Requests != 1 {
		t.Fatalf("%+v", snap)
	}
}

func TestDegradedNoteFromErrors(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 15)
	snap, err := Build(nil, nil, []string{"trae: 登录态在加密存储中，没有可读的 JWT 文件"}, Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if len(snap.Notes) == 0 || !strings.Contains(snap.Notes[0], "Trae") {
		t.Fatalf("notes=%v", snap.Notes)
	}
	for _, n := range snap.Notes {
		if strings.Contains(n, "JWT") && strings.Contains(strings.ToLower(n), "eyj") {
			t.Fatalf("leaked jwt in %q", n)
		}
	}
}

func isUsage(err error) bool {
	type u interface{ Usage() bool }
	if v, ok := err.(interface{ Usage() bool }); ok {
		return v.Usage()
	}
	return false
}

func TestAggregateConservation(t *testing.T) {
	loc := shanghai()
	events, turns := fixture(loc)
	sum := metric.Aggregate(events, turns)
	if sum.All.Total() != 11_680_000 {
		t.Fatalf("total=%d", sum.All.Total())
	}
}
