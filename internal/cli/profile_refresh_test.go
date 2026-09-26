package cli

import (
	"io"
	"net"
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
	"github.com/rainhuang0220/whereToken/internal/proclock"
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
	if _, err := Parse([]string{"profile", "refresh", "--json"}); err == nil {
		t.Fatal("refresh accepted --json")
	}
	if _, err := Parse([]string{"--tool", "claude", "profile", "refresh"}); err == nil {
		t.Fatal("refresh accepted a tool filter")
	}
	if _, err := Parse([]string{"profile", "refresh", "--claude"}); err == nil {
		t.Fatal("refresh accepted a tool shorthand")
	}
}

func TestProfileRefreshSwitchFileStaysMinimal(t *testing.T) {
	home := testhome.New(t.TempDir())
	launchHome := t.TempDir()
	app, out, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = home
	useLaunchd(t, app, launchHome)
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
	useLaunchd(t, app, launchHome)
	if code := app.Run(); code != ExitOK || !strings.Contains(out.String(), "phase=idle 等待下一次") || !strings.Contains(out.String(), "last=\n") {
		t.Fatalf("status %d %s %s", code, out.String(), errb.String())
	}
	app, out, errb = testApp([]string{"profile", "refresh", "off"})
	app.Home = home
	useLaunchd(t, app, launchHome)
	if code := app.Run(); code != ExitOK || !strings.Contains(out.String(), "public profile refresh off (GitHub profile stays as last published)") {
		t.Fatalf("off %d %s", code, out.String())
	}
}

func TestProfileRefreshDoctorAndReportDoNotPut(t *testing.T) {
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

	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	puts := 0
	app, out, errb = testApp([]string{"--quiet"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		puts++
		t.Errorf("report called %s %s", req.Method, req.URL.Path)
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("report %d %s", code, errb.String())
	}
	if puts != 0 {
		t.Fatalf("puts %d", puts)
	}
	combined := out.String() + errb.String()
	if strings.Contains(combined, "phase=") || strings.Contains(combined, "PUBLISHED") || strings.Contains(combined, "用量已更新") {
		t.Fatalf("report printed a refresh line:\n%s", combined)
	}
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
			return jsonRes(200, map[string]any{"ok": true, "readme": "materialized"})
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
	if !strings.Contains(out.String(), "phase=published 用量已更新") || !strings.Contains(out.String(), "VERIFIED") {
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

func TestProfileRefreshOffIsDurableAndOneShotStillPublishes(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	creds := &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	launchHome := t.TempDir()
	app, _, errb := testApp([]string{"profile", "refresh", "on"})
	app.Home = home
	useLaunchd(t, app, launchHome)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("on %d %s", code, errb.String())
	}
	app, _, errb = testApp([]string{"profile", "refresh", "off"})
	app.Home = home
	useLaunchd(t, app, launchHome)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("off %d %s", code, errb.String())
	}
	on, err := publicprofile.LoadRefreshSwitch(publicprofile.RefreshConfigPath(home))
	if err != nil || on {
		t.Fatalf("switch stayed on: %v %v", on, err)
	}
	puts := 0
	app, _, errb = testApp([]string{"profile", "refresh", "watch"})
	app.Home = home
	app.Creds = creds
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			puts++
		}
		return jsonRes(http.StatusNotFound, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("watch %d %s", code, errb.String())
	}
	if puts != 0 {
		t.Fatalf("watch after off put %d", puts)
	}
	app, out, errb := testApp([]string{"profile", "refresh"})
	app.Home = home
	app.Creds = creds
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Scan = func(adapter.Home) scan.Result {
		ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: 12_000, Quality: event.QualityAuthoritative}
		return scan.Result{Events: []event.UsageEvent{ev}}
	}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Header.Get("Authorization") != "" && strings.Contains(req.URL.RawQuery, "wtd_1") {
			t.Fatal("token leaked into the URL")
		}
		if req.Method == http.MethodGet {
			return jsonRes(http.StatusNotFound, nil)
		}
		if req.Method == http.MethodPut {
			puts++
			return jsonRes(200, map[string]any{"ok": true})
		}
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("one-shot %d %s", code, errb.String())
	}
	if puts != 1 || !strings.Contains(out.String(), "PUBLISHED") {
		t.Fatalf("puts=%d %s", puts, out.String())
	}
	if creds.m[credstore.KeyDeviceToken] != "wtd_1.tok" {
		t.Fatal("one-shot deleted the device token")
	}
}

func TestProfileRefreshAuthAndRateLimitDoNotDeleteTokenOrLoop(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	for _, status := range []int{http.StatusUnauthorized, http.StatusForbidden, http.StatusTooManyRequests} {
		home := testhome.New(t.TempDir())
		creds := &memCreds{m: map[string]string{
			credstore.KeyDeviceToken: "wtd_1.secret",
			credstore.KeyLogin:       "rainhuang0220",
		}}
		calls := 0
		app, out, errb := testApp([]string{"profile", "refresh"})
		app.Home = home
		app.Creds = creds
		app.Now = func() time.Time { return now }
		app.Loc = time.UTC
		app.Scan = func(adapter.Home) scan.Result {
			ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: 12_000, Quality: event.QualityAuthoritative}
			return scan.Result{Events: []event.UsageEvent{ev}}
		}
		app.HTTPDo = func(req *http.Request) (*http.Response, error) {
			calls++
			if strings.Contains(req.Header.Get("Authorization"), "wtd_1.secret") && calls > 4 {
				t.Fatal("tight retry")
			}
			return jsonRes(status, map[string]any{"error": "wtd_1.secret"})
		}
		code := app.Run()
		combined := out.String() + errb.String()
		if strings.Contains(combined, "wtd_1.secret") || strings.Contains(combined, "Bearer") {
			t.Fatalf("status %d leaked a credential\n%s", status, combined)
		}
		if creds.m[credstore.KeyDeviceToken] != "wtd_1.secret" {
			t.Fatalf("status %d deleted the token", status)
		}
		if calls > 3 {
			t.Fatalf("status %d calls=%d", status, calls)
		}
		if status == http.StatusTooManyRequests && code != ExitFail {
			t.Fatalf("429 exit %d %s", code, combined)
		}
		if status != http.StatusTooManyRequests && code != ExitOK {
			t.Fatalf("%d exit %d %s", status, code, combined)
		}
	}
}

func TestProfileRefreshHTTPClientTimesOut(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		time.Sleep(2 * time.Second)
		_ = conn.Close()
	}()
	old := profileRefreshTimeout
	profileRefreshTimeout = 200 * time.Millisecond
	defer func() { profileRefreshTimeout = old }()
	home := testhome.New(t.TempDir())
	app, _, errb := testApp([]string{"profile", "refresh"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_HOSTED_URL" {
			return "http://" + ln.Addr().String()
		}
		return ""
	}
	app.Scan = func(adapter.Home) scan.Result {
		ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: time.Date(2026, 9, 25, 14, 0, 0, 0, time.UTC), Miss: 12_000, Quality: event.QualityAuthoritative}
		return scan.Result{Events: []event.UsageEvent{ev}}
	}
	start := time.Now()
	code := app.Run()
	if code != ExitFail {
		t.Fatalf("exit %d %s", code, errb.String())
	}
	if time.Since(start) > 2*time.Second {
		t.Fatalf("refresh waited %s without a client timeout", time.Since(start))
	}
	if strings.Contains(errb.String(), "wtd_1") || strings.Contains(errb.String(), "Bearer") {
		t.Fatalf("timeout log leaked a credential: %s", errb.String())
	}
}

func TestProfileRefreshSecondLockSkipsWithoutPut(t *testing.T) {
	home := testhome.New(t.TempDir())
	app, out, errb := testApp([]string{"profile", "refresh"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	app.Scan = func(adapter.Home) scan.Result {
		t.Fatal("scanned while another refresh holds the lock")
		return scan.Result{}
	}
	puts := 0
	app.HTTPDo = func(*http.Request) (*http.Response, error) {
		puts++
		return jsonRes(500, nil)
	}
	release, ok, err := proclock.TryLock(app.scanLockPath(home))
	if err != nil || !ok {
		t.Fatalf("lock ok=%v err=%v", ok, err)
	}
	defer release()
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code %d %s", code, errb.String())
	}
	if puts != 0 || !strings.Contains(out.String(), "phase=skipped 已有扫描，跳过本次") {
		t.Fatalf("puts=%d %s", puts, out.String())
	}
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(home))
	if err != nil || st.Code != publicprofile.CodeBusy {
		t.Fatalf("state %+v %v", st, err)
	}
	raw, err := os.ReadFile(publicprofile.RefreshStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "0.00 M") || strings.Contains(string(raw), "total") || strings.Contains(string(raw), home.XDGConfig("wheretoken")) {
		t.Fatalf("state leaked %s", raw)
	}
}

func TestProfileRefreshStateCheckedAtIsTheRunInstant(t *testing.T) {
	home := testhome.New(t.TempDir())
	now := time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)
	app, out, errb := testApp([]string{"profile", "refresh", "--quiet"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	app.Now = func() time.Time { return now }
	app.Loc = time.UTC
	app.Scan = func(adapter.Home) scan.Result {
		ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: 12_000, Quality: event.QualityAuthoritative}
		return scan.Result{Events: []event.UsageEvent{ev}}
	}
	app.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodGet {
			return jsonRes(http.StatusNotFound, nil)
		}
		if req.Method == http.MethodPut {
			return jsonRes(200, map[string]any{"ok": true})
		}
		return jsonRes(500, nil)
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code %d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "PUBLISHED") {
		t.Fatalf("%s", out.String())
	}
	raw, err := os.ReadFile(publicprofile.RefreshStatePath(home))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	if strings.Contains(text, "T00:00:00") {
		t.Fatalf("state claims a midnight publish: %s", text)
	}
	if !strings.Contains(text, "2026-09-25T08:00:00Z") || !strings.Contains(text, `"code": "PUBLISHED"`) {
		t.Fatalf("state %s", text)
	}
	for _, bad := range []string{"total", "snapshot", "0.00 M", "$0", "#0"} {
		if strings.Contains(text, bad) {
			t.Fatalf("state contains %q: %s", bad, text)
		}
	}
}

func TestProfileRefreshWatchRereadsSwitch(t *testing.T) {
	home := testhome.New(t.TempDir())
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	old := profileRefreshInterval
	profileRefreshInterval = 20 * time.Millisecond
	t.Cleanup(func() { profileRefreshInterval = old })
	app, out, errb := testApp([]string{"profile", "refresh", "watch"})
	app.Home = home
	app.Creds = &memCreds{m: map[string]string{
		credstore.KeyDeviceToken: "wtd_1.tok",
		credstore.KeyLogin:       "rainhuang0220",
	}}
	app.Scan = func(adapter.Home) scan.Result { return scan.Result{} }
	seen := make(chan struct{})
	app.HTTPDo = func(*http.Request) (*http.Response, error) {
		select {
		case <-seen:
		default:
			close(seen)
		}
		return jsonRes(http.StatusNotFound, nil)
	}
	done := make(chan int, 1)
	go func() { done <- app.Run() }()
	select {
	case <-seen:
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not start")
	}
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), false); err != nil {
		t.Fatal(err)
	}
	select {
	case code := <-done:
		if code != ExitOK {
			t.Fatalf("code %d %s %s", code, out.String(), errb.String())
		}
	case <-time.After(2 * time.Second):
		t.Fatal("watch did not notice the switch")
	}
	if !strings.Contains(out.String(), "phase=idle 未开启") {
		t.Fatalf("%s", out.String())
	}
}

func TestDoScanDoesNotContinueWhenLockFails(t *testing.T) {
	root := t.TempDir()
	app, _, _ := testApp(nil)
	app.Scan = nil
	home := testhome.New(root)
	grand := filepath.Dir(filepath.Dir(app.scanLockPath(home)))
	if err := os.MkdirAll(filepath.Dir(grand), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(grand, []byte("not-a-directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	res := app.doScan(home, true, true, false)
	if len(res.Events) != 0 || len(res.Errors) != 1 || res.Errors[0] != "scan lock unavailable" {
		t.Fatalf("%+v", res)
	}
	if strings.Contains(res.Errors[0], root) {
		t.Fatal("lock error included a path")
	}
}
