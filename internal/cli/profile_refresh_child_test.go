package cli

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/proclock"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

// This is an isolated, owned test child: no real adapters, credentials or API.
func TestProfileRefreshWatchdogHelper(t *testing.T) {
	root := os.Getenv("WHERETOKEN_REFRESH_WATCHDOG_TEST_HOME")
	if root == "" {
		return
	}
	profileRefreshRunTimeout = 50 * time.Millisecond
	home := testhome.New(root)
	app, _, _ := testApp([]string{"profile", "refresh"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{credstore.KeyDeviceToken: "wtd_1.fixture", credstore.KeyLogin: "fixture"}}
	app.Scan = func(adapter.Home) scan.Result {
		fmt.Fprintln(os.Stdout, "owned scan started")
		time.Sleep(time.Hour)
		return scan.Result{}
	}
	os.Exit(app.Run())
}

func TestProfileRefreshChildWatchdogSurvivesWithoutWatchParent(t *testing.T) {
	root := t.TempDir()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, exe, "-test.run=^TestProfileRefreshWatchdogHelper$")
	cmd.Env = append(os.Environ(), profileRefreshChildEnv+"=1", "WHERETOKEN_REFRESH_WATCHDOG_TEST_HOME="+root)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) || exitErr.ExitCode() != ExitFail || ctx.Err() != nil {
		t.Fatalf("owned worker did not enforce its own bound: err=%v context=%v output=%s", err, ctx.Err(), out.String())
	}
	if !bytes.Contains(out.Bytes(), []byte("owned scan started")) {
		t.Fatalf("fixture never held lock: %s", out.String())
	}
	home := testhome.New(root)
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
	if err != nil || st.Code != publicprofile.CodeWillRetry {
		t.Fatalf("timeout state=%+v err=%v", st, err)
	}
	app := &App{}
	release, ok, err := proclock.TryLock(app.scanLockPath(home))
	if err != nil || !ok {
		t.Fatalf("terminated child's lock remained held: ok=%v err=%v", ok, err)
	}
	release()
}

func TestProfileRefreshCancelHelper(t *testing.T) {
	path := os.Getenv("WHERETOKEN_REFRESH_CANCEL_TEST_LOCK")
	if path == "" {
		return
	}
	release, ok, err := proclock.TryLock(path)
	if err != nil || !ok {
		os.Exit(90)
	}
	defer release()
	fmt.Fprintln(os.Stdout, "ready")
	time.Sleep(2 * time.Second)
	os.Exit(99)
}

type cancelRefreshOnReady struct{ cancel context.CancelFunc }

func (w cancelRefreshOnReady) Write(p []byte) (int, error) {
	if bytes.Contains(p, []byte("ready")) {
		w.cancel()
	}
	return len(p), nil
}

func TestOwnedRefreshCommandCancellationReapsChildAndReleasesLock(t *testing.T) {
	lock := filepath.Join(t.TempDir(), "scan.lock")
	t.Setenv("WHERETOKEN_REFRESH_CANCEL_TEST_LOCK", lock)
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	app := &App{Stdout: cancelRefreshOnReady{cancel}, Stderr: io.Discard}
	start := time.Now()
	err = app.execProfileRefreshChild(ctx, exe, []string{"-test.run=^TestProfileRefreshCancelHelper$"})
	if err == nil || !errors.Is(ctx.Err(), context.Canceled) || time.Since(start) > time.Second {
		t.Fatalf("child was not promptly cancelled and reaped: err=%v context=%v elapsed=%s", err, ctx.Err(), time.Since(start))
	}
	release, ok, err := proclock.TryLock(lock)
	if err != nil || !ok {
		t.Fatalf("owned child left scan.lock held: ok=%v err=%v", ok, err)
	}
	release()
}
