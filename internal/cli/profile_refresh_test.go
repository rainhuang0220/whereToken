package cli

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestParseProfileRefresh(t *testing.T) {
	f, err := Parse([]string{"profile", "refresh"})
	if err != nil || f.ProfileAction != "refresh" || f.ProfileRefreshAction != "" {
		t.Fatalf("%+v %v", f, err)
	}
	for _, action := range []string{"status", "on", "off", "watch"} {
		f, err = Parse([]string{"profile", "refresh", action})
		if err != nil || f.ProfileRefreshAction != action {
			t.Fatalf("%s %+v %v", action, f, err)
		}
	}
	f, err = Parse([]string{"--offline", "--quiet", "profile", "refresh"})
	if err != nil || !f.Offline || !f.Quiet || f.ProfileAction != "refresh" {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := Parse([]string{"profile", "refresh", "publish"}); err == nil {
		t.Fatal("unknown refresh action was accepted")
	}
	if _, err := Parse([]string{"profile", "refresh", "--today"}); err == nil {
		t.Fatal("refresh accepted a window")
	}
}

func TestProfileRefreshSwitchFileStaysMinimal(t *testing.T) {
	home := testhome.New(t.TempDir())
	app, out, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = home
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "public profile refresh on (sanitized snapshot only — prompts and paths stay local)") {
		t.Fatalf("on text %s", out.String())
	}
	if strings.Contains(out.String(), "profile publish") {
		t.Fatal("on told the user to profile publish")
	}
	path := publicprofile.RefreshConfigPath(home)
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if !strings.Contains(text, `"schema_version": 1`) || !strings.Contains(text, `"enabled": true`) {
		t.Fatalf("switch %s", text)
	}
	for _, bad := range []string{"token", "snapshot", "path", "total", "/Users", "home"} {
		if strings.Contains(strings.ToLower(text), bad) {
			t.Fatalf("switch copied %q: %s", bad, text)
		}
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %o", st.Mode().Perm())
		}
		dir, err := os.Stat(filepath.Dir(path))
		if err != nil {
			t.Fatal(err)
		}
		if dir.Mode().Perm() != 0o700 {
			t.Fatalf("dir mode %o", dir.Mode().Perm())
		}
	}
	app, out, errb = testApp([]string{"profile", "refresh", "status"})
	app.Home = home
	if code := app.Run(); code != ExitOK || !strings.Contains(out.String(), "phase=idle 等待下一次") {
		t.Fatalf("status %d %s %s", code, out.String(), errb.String())
	}
	app, out, errb = testApp([]string{"profile", "refresh", "off"})
	app.Home = home
	if code := app.Run(); code != ExitOK || !strings.Contains(out.String(), "public profile refresh off (GitHub profile stays as last published)") {
		t.Fatalf("off %d %s", code, out.String())
	}
}

func TestProfileRefreshDoctorAndOptedOutReportDoNotPut(t *testing.T) {
	dir := t.TempDir()
	home := testhome.New(dir)
	app, out, errb := testApp([]string{"doctor"})
	app.Home = home
	if code := app.Run(); code != ExitOK {
		t.Fatalf("doctor %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Public profile refresh") || !strings.Contains(out.String(), "Off") {
		t.Fatalf("doctor\n%s", out.String())
	}
	if _, err := os.Stat(publicprofile.RefreshConfigPath(home)); !os.IsNotExist(err) {
		t.Fatal("doctor created the refresh switch")
	}

	puts := 0
	app, out, errb = testApp([]string{"--quiet"})
	app.Home = home
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		puts++
		t.Errorf("opted-out report called %s %s", req.Method, req.URL.Path)
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("report %d %s", code, errb.String())
	}
	if puts != 0 {
		t.Fatalf("puts %d", puts)
	}
	if strings.Contains(errb.String(), "phase=") {
		t.Fatalf("opted-out report wrote a refresh line: %s", errb.String())
	}
	_ = out
}

func TestProfileRefreshOneShotPutsAndUnavailableDoesNot(t *testing.T) {
	dir := t.TempDir()
	home := testhome.New(dir)
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	creds := &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	puts := 0
	var sawBatch bool
	app, out, errb := testApp([]string{"profile", "refresh", "--quiet"})
	app.Home = home
	app.Creds = creds
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Scan = func(adapter.Home) scan.Result {
		ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: 12_000, Quality: event.QualityAuthoritative}
		return scan.Result{Events: []event.UsageEvent{ev}}
	}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if strings.Contains(req.URL.Path, "/sync/batch") {
			sawBatch = true
		}
		if req.Method == http.MethodGet {
			return jsonRes(http.StatusNotFound, nil)
		}
		if req.Method == http.MethodPut && strings.HasSuffix(req.URL.Path, "/sync/public-profile") {
			puts++
			body, _ := io.ReadAll(req.Body)
			if strings.Contains(string(body), `"live_sync": true`) {
				t.Fatal("refresh set live_sync")
			}
			return jsonRes(200, map[string]any{"ok": true, "readme": "coalesced"})
		}
		t.Errorf("unexpected %s %s", req.Method, req.URL.Path)
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("refresh %d %s", code, errb.String())
	}
	if puts != 1 || sawBatch {
		t.Fatalf("puts=%d batch=%v", puts, sawBatch)
	}
	if !strings.Contains(out.String(), "phase=published 用量已更新") || !strings.Contains(out.String(), "PUBLISHED") {
		t.Fatalf("stdout %s", out.String())
	}
	if _, err := os.Stat(publicprofile.RefreshConfigPath(home)); !os.IsNotExist(err) {
		t.Fatal("one-shot created the refresh switch")
	}

	puts = 0
	app, out, errb = testApp([]string{"profile", "refresh"})
	app.Home = home
	app.Creds = creds
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Scan = func(adapter.Home) scan.Result { return scan.Result{} }
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			puts++
		}
		if req.Method == http.MethodGet {
			return jsonRes(http.StatusNotFound, nil)
		}
		return jsonRes(200, map[string]any{"ok": true})
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("unavailable %d %s", code, errb.String())
	}
	combined := out.String() + errb.String()
	if puts != 0 {
		t.Fatalf("unavailable puts %d\n%s", puts, combined)
	}
	if strings.Contains(combined, "0.00 M") || strings.Contains(combined, "$0") || strings.Contains(combined, "#0") {
		t.Fatalf("zero display\n%s", combined)
	}
}

func TestProfileRefreshNotLoggedInSkips(t *testing.T) {
	app, _, errb := testApp([]string{"profile", "refresh"})
	app.Home = testhome.New(t.TempDir())
	app.Creds = &memCreds{m: map[string]string{}}
	called := false
	app.HTTPDo = func(*http.Request) (*http.Response, error) {
		called = true
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code %d %s", code, errb.String())
	}
	if called || !strings.Contains(errb.String(), "not logged in; run wheretoken login") {
		t.Fatalf("called=%v %s", called, errb.String())
	}
}

func TestProfileRefreshWatchOffDoesNotLoop(t *testing.T) {
	app, out, errb := testApp([]string{"profile", "refresh", "watch"})
	app.Home = testhome.New(t.TempDir())
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "phase=idle 未开启") {
		t.Fatalf("%s", out.String())
	}
}
