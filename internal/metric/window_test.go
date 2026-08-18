package metric

import (
	"strings"
	"testing"
	"time"
)

func TestParseWindowSinceSevenDays(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 8, 19, 15, 0, 0, 0, loc)
	w, err := ParseWindow(false, "7d", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if w.Days != 7 || w.Label != "近 7 天" {
		t.Fatalf("%+v", w)
	}
	if w.From.Format("2006-01-02") != "2026-08-13" {
		t.Fatalf("from=%s", w.From)
	}
	if !w.Contains(time.Date(2026, 8, 13, 0, 0, 0, 0, loc), loc) {
		t.Fatal("first day of window")
	}
	if w.Contains(time.Date(2026, 8, 12, 23, 0, 0, 0, loc), loc) {
		t.Fatal("day before window")
	}
}

func TestParseSinceErrorMentionsNd(t *testing.T) {
	_, err := ParseSince("nope")
	if err == nil || !strings.Contains(err.Error(), "Nd") {
		t.Fatalf("err=%v", err)
	}
}

func TestParseWindowTodayAndSinceConflict(t *testing.T) {
	_, err := ParseWindow(true, "7d", "", "", time.Now(), time.Local)
	if err == nil {
		t.Fatal("expected conflict")
	}
}

func TestParseWindowFromTo(t *testing.T) {
	loc := time.UTC
	w, err := ParseWindow(false, "", "2026-08-01", "2026-08-02", time.Date(2026, 8, 19, 0, 0, 0, 0, loc), loc)
	if err != nil {
		t.Fatal(err)
	}
	if !w.Contains(time.Date(2026, 8, 1, 12, 0, 0, 0, loc), loc) {
		t.Fatal("from inclusive")
	}
	if w.Contains(time.Date(2026, 8, 3, 0, 0, 0, 0, loc), loc) {
		t.Fatal("to date is inclusive of 2026-08-02 only")
	}
}

func TestWindowPrevious(t *testing.T) {
	loc := time.UTC
	w, err := ParseWindow(false, "7d", "", "", time.Date(2026, 8, 19, 12, 0, 0, 0, loc), loc)
	if err != nil {
		t.Fatal(err)
	}
	p := w.Previous()
	if p.To != w.From {
		t.Fatalf("prev to=%s want %s", p.To, w.From)
	}
}

func TestNewCompareDelta(t *testing.T) {
	cur := Summary{All: Slice{Miss: 132}, BySource: []Slice{{ID: "claude", Label: "Claude Code", Miss: 112}, {ID: "kimi", Label: "Kimi Code", Miss: 20}}}
	prev := Summary{All: Slice{Miss: 100}, BySource: []Slice{{ID: "claude", Label: "Claude Code", Miss: 100}}}
	c := NewCompare(cur, prev)
	if c.PreviousTotal != 100 || c.DeltaPct == nil || *c.DeltaPct < 31 || *c.DeltaPct > 33 {
		t.Fatalf("%+v", c)
	}
}
