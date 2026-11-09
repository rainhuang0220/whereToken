package cli

import (
	"context"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/proclock"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestProfileRefreshWatchRunsOwnedChildWithDeadline(t *testing.T) {
	home := testhome.New(t.TempDir())
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp([]string{"profile", "refresh", "watch"})
	app.Home = home
	app.UserHome = func() (string, error) { return t.TempDir(), nil }
	app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
	app.Reexec = func([]string) {}
	app.Creds = &memCreds{m: map[string]string{credstore.KeyDeviceToken: "wtd_1.fixture", credstore.KeyLogin: "fixture"}}
	scans, children := 0, 0
	stop := func() {
		if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
			t.Fatal(err)
		}
	}
	app.Scan = func(adapter.Home) scan.Result { scans++; stop(); return scan.Result{} }
	app.HTTPDo = func(*http.Request) (*http.Response, error) { return jsonRes(404, nil) }
	app.RunRefreshChild = func(ctx context.Context, program string, args []string) error {
		children++
		if program != "/fixture/wheretoken" {
			t.Fatalf("program=%q", program)
		}
		if strings.Join(args, " ") != "profile refresh --quiet" {
			t.Fatalf("child must be a one-shot: %v", args)
		}
		deadline, ok := ctx.Deadline()
		if !ok || time.Until(deadline) <= 0 || time.Until(deadline) > 5*time.Minute {
			t.Fatalf("missing five-minute total bound: %v %v", deadline, ok)
		}
		stop()
		return nil
	}
	// An immediate switch change must be observed without waiting for the next
	// calendar slot; the old in-process loop uses this interval in isolated tests.
	old := profileRefreshInterval
	profileRefreshInterval = time.Millisecond
	defer func() { profileRefreshInterval = old }()
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if scans != 0 || children != 1 {
		t.Fatalf("watch scanned in-process: scans=%d children=%d", scans, children)
	}
}

func TestProfileRefreshWatchRetriesBusyWithBoundedBackoff(t *testing.T) {
	old := profileRefreshInterval
	profileRefreshInterval = time.Millisecond
	defer func() { profileRefreshInterval = old }()
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC)
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	app, _, _ := testApp([]string{"profile", "refresh", "watch"})
	app.Home = home
	app.UserHome = func() (string, error) { return t.TempDir(), nil }
	app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Reexec = func([]string) {}
	attempts := 0
	var waits []time.Duration
	app.RunRefreshChild = func(context.Context, string, []string) error {
		attempts++
		if attempts > 3 {
			t.Fatal("more than three attempts in one slot")
		}
		return publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{
			Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeBusy, CheckedAt: now,
		})
	}
	app.Sleep = func(d time.Duration) {
		waits = append(waits, d)
		if len(waits) > 4 {
			t.Fatal("retry loop did not stop")
		}
		if attempts == 3 {
			if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
				t.Fatal(err)
			}
			return
		}
		now = now.Add(d)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if attempts != 3 {
		t.Fatalf("attempts=%d", attempts)
	}
	if len(waits) != 4 || waits[0] != 30*time.Second || waits[1] != time.Minute || waits[2] != time.Minute || waits[3] != time.Minute {
		t.Fatalf("wrong bounded retry/backoff: %v", waits)
	}
}

func TestProfileRefreshRetryBudgetSurvivesWatchRestart(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 11, 9, 17, 0, 0, 0, time.UTC)
	attempts := 0
	for run := 0; run < 6; run++ {
		if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
			t.Fatal(err)
		}
		app, _, _ := testApp([]string{"profile", "refresh", "watch"})
		app.Home = home
		app.UserHome = func() (string, error) { return t.TempDir(), nil }
		app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
		app.Now = func() time.Time { return now }
		app.Loc = time.UTC
		app.Reexec = func([]string) {}
		app.RunRefreshChild = func(context.Context, string, []string) error {
			attempts++
			return publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{
				Phase: publicprofile.PhaseFailed, Code: publicprofile.CodeWillRetry, CheckedAt: now,
			})
		}
		app.Sleep = func(d time.Duration) {
			if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
				t.Fatal(err)
			}
			if d <= 2*time.Minute {
				now = now.Add(d)
			}
		}
		if code := app.Run(); code != ExitOK {
			t.Fatalf("run=%d code=%d", run, code)
		}
	}
	if attempts != 3 {
		t.Fatalf("restart reset the per-slot budget: attempts=%d", attempts)
	}
}

func TestProfileRefreshSecondWatchDoesNotSpendAttemptBudget(t *testing.T) {
	home := testhome.New(t.TempDir())
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	lock := filepath.Join(filepath.Dir(publicprofile.RefreshConfigPath(home)), "profile-refresh-watch.lock")
	release, ok, err := proclock.TryLock(lock)
	if err != nil || !ok {
		t.Fatalf("fixture lock=%v err=%v", ok, err)
	}
	defer release()
	app, _, _ := testApp([]string{"profile", "refresh", "watch"})
	app.Home = home
	app.UserHome = func() (string, error) { return t.TempDir(), nil }
	app.RunRefreshChild = func(context.Context, string, []string) error { t.Fatal("second watch started a child"); return nil }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if _, err := os.Stat(profileRefreshSchedulePath(home)); !os.IsNotExist(err) {
		t.Fatalf("second watch spent schedule budget: %v", err)
	}
}

func TestProfileRefreshIdleWatchCapsClockChecksWithoutScanning(t *testing.T) {
	for _, tc := range []struct {
		name    string
		now     time.Time
		checked time.Time
		want    time.Duration
	}{
		{"long sleep", time.Date(2026, 11, 9, 10, 31, 0, 0, time.UTC), time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC), time.Minute},
		{"near slot", time.Date(2026, 11, 9, 10, 29, 30, 0, time.UTC), time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC), 30 * time.Second},
	} {
		t.Run(tc.name, func(t *testing.T) {
			home := testhome.New(t.TempDir())
			if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
				t.Fatal(err)
			}
			if err := publicprofile.SaveRefreshState(publicprofile.RefreshStatePath(home), publicprofile.RefreshState{Phase: publicprofile.PhaseSkipped, Code: publicprofile.CodeSkippedDelta, CheckedAt: tc.checked}); err != nil {
				t.Fatal(err)
			}
			app, _, _ := testApp([]string{"profile", "refresh", "watch"})
			app.Home = home
			app.Now = func() time.Time { return tc.now }
			app.Loc = time.UTC
			app.UserHome = func() (string, error) { return t.TempDir(), nil }
			app.RunRefreshChild = func(context.Context, string, []string) error { t.Fatal("idle watch started a source scan"); return nil }
			waits := 0
			app.Sleep = func(d time.Duration) {
				waits++
				if d != tc.want {
					t.Fatalf("clock check wait=%s want=%s", d, tc.want)
				}
				if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
					t.Fatal(err)
				}
			}
			if code := app.Run(); code != ExitOK || waits != 1 {
				t.Fatalf("code=%d waits=%d", code, waits)
			}
		})
	}
}

func TestScheduledRefreshCancellationLeavesSlotRetryable(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 11, 9, 10, 30, 0, 0, time.UTC)
	app, _, _ := testApp(nil)
	app.Home = home
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Executable = func() (string, error) { return "/fixture/wheretoken", nil }
	app.RunRefreshChild = func(ctx context.Context, _ string, _ []string) error { return ctx.Err() }
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	st := profileRefreshSchedule{Slot: now}
	if err := app.attemptScheduledProfileRefresh(ctx, Flags{}, home, st); err != nil {
		t.Fatal(err)
	}
	now = now.Add(31 * time.Second)
	got, next, err := app.refreshSchedule(home, now)
	if err != nil || got.Done || got.Attempts != 1 || next.After(now) {
		t.Fatalf("cancelled scan consumed slot: state=%+v next=%s err=%v", got, next, err)
	}
}
