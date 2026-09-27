package publicprofile

import (
	"testing"
	"time"
)

func TestNextRefreshWakePrefersMidnight(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 9, 24, 23, 50, 0, 0, loc)
	got := NextRefreshWake(now, loc, 15*time.Minute)
	want := time.Date(2026, 9, 25, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("wake %s want %s", got, want)
	}
	after := NextRefreshWake(want, loc, 15*time.Minute)
	if !after.Equal(want.Add(15 * time.Minute)) {
		t.Fatalf("after midnight %s", after)
	}
}

func TestNextRefreshWakeOrdinaryInterval(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, loc)
	got := NextRefreshWake(now, loc, 15*time.Minute)
	if !got.Equal(now.Add(15 * time.Minute)) {
		t.Fatalf("interval %s", got)
	}
}

func TestNextRefreshWakeDSTSpringAndFall(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	spring := time.Date(2026, 3, 7, 23, 50, 0, 0, loc)
	got := NextRefreshWake(spring, loc, 15*time.Minute)
	want := time.Date(2026, 3, 8, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("spring %s want %s", got, want)
	}
	fall := time.Date(2026, 10, 31, 23, 50, 0, 0, loc)
	got = NextRefreshWake(fall, loc, 15*time.Minute)
	want = time.Date(2026, 11, 1, 0, 0, 0, 0, loc)
	if !got.Equal(want) {
		t.Fatalf("fall %s want %s", got, want)
	}
	// The repeated hour is 01:00, not midnight. A wake inside it still
	// targets the next calendar midnight, not a second 00:00.
	repeated := time.Date(2026, 11, 1, 1, 30, 0, 0, loc)
	got = NextRefreshWake(repeated, loc, 15*time.Minute)
	if !got.Equal(repeated.Add(15 * time.Minute)) {
		t.Fatalf("repeated hour %s", got)
	}
}

func TestNextRefreshWakeSleepCatchUpIsTheNextInstant(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	slept := time.Date(2026, 9, 25, 8, 0, 0, 0, loc)
	got := NextRefreshWake(slept, loc, 15*time.Minute)
	if !got.Equal(slept.Add(15 * time.Minute)) {
		t.Fatalf("wake after sleep %s", got)
	}
	if got.In(loc).Format("2006-01-02") != "2026-09-25" {
		t.Fatal("catch-up invented another local date")
	}
}
