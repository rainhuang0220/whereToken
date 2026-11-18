package cli

import (
	"context"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

// Virtual clock only. A network-class failure keeps retrying for the whole
// horizon and never stops after the old three-attempt slot cap. A later
// success clears the budget. No sockets and no wall clock.
func TestProfileRefreshOutageRetriesAcrossHorizons(t *testing.T) {
	for _, days := range []int{7, 14, 30} {
		t.Run(time.Duration(days).String(), func(t *testing.T) {
			attempts := simulateRefreshOutage(t, days)
			// The old production cap was three attempts, then silence until the
			// next calendar slot. The long backoff is four hours, so a horizon
			// stays well above three and well below a one-minute loop.
			if attempts < days*4 || attempts > days*8+6 {
				t.Fatalf("days=%d attempts=%d", days, attempts)
			}
		})
	}
}

func simulateRefreshOutage(t *testing.T, days int) int {
	t.Helper()
	home := testhome.New(t.TempDir())
	start := time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
	now := start
	end := start.AddDate(0, 0, days)
	app, _, _ := testApp(nil)
	app.Home = home
	app.Loc = time.UTC
	app.Now = func() time.Time { return now }
	app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
	app.RunRefreshChild = func(context.Context, string, []string) error {
		return publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{
			Phase: publicprofile.PhaseFailed, Code: publicprofile.CodeWillRetry, CheckedAt: now,
		})
	}
	attempts := 0
	var previous time.Time
	for now.Before(end) {
		st, wake, err := app.refreshSchedule(home, now)
		if err != nil {
			t.Fatal(err)
		}
		if wake.After(now) {
			now = wake
			continue
		}
		before := now
		if err := app.attemptScheduledProfileRefresh(context.Background(), Flags{}, home, st); err != nil {
			t.Fatal(err)
		}
		attempts++
		saved, _, err := app.refreshSchedule(home, now)
		if err != nil {
			t.Fatal(err)
		}
		if attempts >= 4 {
			gap := saved.RetryAt.Sub(before)
			if gap < 2*time.Hour || gap > 5*time.Hour {
				t.Fatalf("attempt %d delay %s", attempts, gap)
			}
		}
		if !previous.IsZero() && !saved.RetryAt.After(previous) {
			t.Fatalf("retry did not move forward: %s then %s", previous, saved.RetryAt)
		}
		previous = saved.RetryAt
		if !saved.RetryAt.After(now) {
			t.Fatal("failed attempt left no retry deadline")
		}
		now = saved.RetryAt
		if attempts > days*8 {
			t.Fatalf("retry storm attempts=%d", attempts)
		}
	}
	return attempts
}

func TestProfileRefreshRecoveryClearsTheOutageBudget(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 1, 1, 10, 30, 0, 0, time.UTC)
	app, _, _ := testApp(nil)
	app.Home = home
	app.Loc = time.UTC
	app.Now = func() time.Time { return now }
	app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
	failed := true
	app.RunRefreshChild = func(context.Context, string, []string) error {
		code := publicprofile.CodeVerified
		phase := publicprofile.PhasePublished
		if failed {
			code = publicprofile.CodeWillRetry
			phase = publicprofile.PhaseFailed
		}
		return publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{
			Phase: phase, Code: code, CheckedAt: now,
		})
	}
	st, _, err := app.refreshSchedule(home, now)
	if err != nil {
		t.Fatal(err)
	}
	if err := app.attemptScheduledProfileRefresh(context.Background(), Flags{}, home, st); err != nil {
		t.Fatal(err)
	}
	failed = false
	st, wake, err := app.refreshSchedule(home, now)
	if err != nil {
		t.Fatal(err)
	}
	now = wake
	st, wake, err = app.refreshSchedule(home, now)
	if err != nil || wake.After(now) {
		t.Fatalf("retry was not due: %v %v", wake, err)
	}
	if err := app.attemptScheduledProfileRefresh(context.Background(), Flags{}, home, st); err != nil {
		t.Fatal(err)
	}
	_, wake, err = app.refreshSchedule(home, now.Add(time.Second))
	if err != nil {
		t.Fatal(err)
	}
	next := publicprofile.NextScheduledRefreshWake(now.Add(time.Second), time.UTC)
	if !wake.Equal(next) {
		t.Fatalf("recovery wake=%s want next slot %s", wake, next)
	}
}
