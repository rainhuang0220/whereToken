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

// Scheduled checks are owner-local calendar instants, not an hourly ticker.
// Exactly three checks per day, with next local midnight after 17:00.
func TestNextScheduledRefreshWakeThreeOwnerLocalSlots(t *testing.T) {
	loc := time.FixedZone("owner", 8*3600)
	for _, tc := range []struct {
		now, next, latest string
	}{
		{"2026-11-09 00:00", "2026-11-09 10:30", "2026-11-09 00:00"},
		{"2026-11-09 10:29", "2026-11-09 10:30", "2026-11-09 00:00"},
		{"2026-11-09 10:30", "2026-11-09 17:00", "2026-11-09 10:30"},
		{"2026-11-09 16:59", "2026-11-09 17:00", "2026-11-09 10:30"},
		{"2026-11-09 17:00", "2026-11-10 00:00", "2026-11-09 17:00"},
		{"2026-11-09 23:59", "2026-11-10 00:00", "2026-11-09 17:00"},
	} {
		parse := func(v string) time.Time {
			t.Helper()
			at, err := time.ParseInLocation("2006-01-02 15:04", v, loc)
			if err != nil {
				t.Fatal(err)
			}
			return at
		}
		now := parse(tc.now)
		if got := NextScheduledRefreshWake(now, loc); !got.Equal(parse(tc.next)) {
			t.Errorf("after %s next=%s want %s", tc.now, got, tc.next)
		}
		if got := ScheduledRefreshSlot(now, loc); !got.Equal(parse(tc.latest)) {
			t.Errorf("at %s latest=%s want %s", tc.now, got, tc.latest)
		}
	}
}

func TestFixedScheduleHandlesDSTDays(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	for _, day := range []string{"2026-03-08", "2026-11-01"} {
		now, err := time.ParseInLocation("2006-01-02 15:04", day+" 00:00", loc)
		if err != nil {
			t.Fatal(err)
		}
		first := NextScheduledRefreshWake(now, loc)
		if first.In(loc).Format("2006-01-02 15:04") != day+" 10:30" {
			t.Errorf("DST day %s first %s", day, first)
		}
		second := NextScheduledRefreshWake(first, loc)
		if second.In(loc).Format("2006-01-02 15:04") != day+" 17:00" {
			t.Errorf("DST day %s second %s", day, second)
		}
		next := NextScheduledRefreshWake(second, loc)
		if next.In(loc).Hour() != 0 || next.In(loc).Day() == second.In(loc).Day() {
			t.Errorf("DST day %s next midnight %s", day, next)
		}
	}
}
