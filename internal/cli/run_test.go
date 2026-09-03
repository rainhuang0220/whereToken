package cli

import (
	"bytes"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func fixtureResult() scan.Result {
	loc := shanghai()
	ts := func(d, hh int) time.Time {
		return time.Date(2026, 8, d, hh, 0, 0, 0, loc)
	}
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Timestamp: ts(15, 10), Miss: 1_000_000, CacheRead: 9_000_000, Output: 100_000, Quality: event.QualityAuthoritative},
		{Source: "claude", Vendor: "minimax", Model: "MiniMax-M3", RequestID: "b", Timestamp: ts(16, 11), Miss: 500_000, Output: 50_000, Quality: event.QualityAuthoritative},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "c", Timestamp: ts(16, 12), Miss: 200_000, CacheRead: 800_000, Output: 30_000, Quality: event.QualityAuthoritative},
	}
	turns := []event.TurnEvent{
		{Source: "claude", Timestamp: ts(15, 10)},
		{Source: "claude", Timestamp: ts(16, 11)},
		{Source: "kimi", Timestamp: ts(16, 12)},
	}
	return scan.Result{
		Summary: metric.Aggregate(events, turns),
		Events:  events,
		Turns:   turns,
		Errors:  []string{},
	}
}

func testApp(args []string) (*App, *bytes.Buffer, *bytes.Buffer) {
	out, errb := &bytes.Buffer{}, &bytes.Buffer{}
	loc := shanghai()
	now := time.Date(2026, 8, 16, 15, 0, 0, 0, loc)
	app := &App{
		Args:      args,
		Stdout:    out,
		Stderr:    errb,
		Version:   "test",
		Now:       func() time.Time { return now },
		Loc:       loc,
		GOOS:      "darwin",
		LookupEnv: func(string) string { return "" },
		Scan: func(adapter.Home) scan.Result {
			return fixtureResult()
		},
	}
	return app, out, errb
}

func TestRunOfflineAddsNote(t *testing.T) {
	app, out, errb := testApp([]string{"--offline"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "offline") || !strings.Contains(s, "本机账本") {
		t.Fatalf("%s", s)
	}
	title := strings.Index(s, "whereToken")
	off := strings.Index(s, "offline ·")
	if title < 0 || off < 0 || off < title {
		t.Fatalf("offline banner should sit under the title:\n%s", s)
	}
}

func TestRunCursorOmitsTraeNote(t *testing.T) {
	app, out, errb := testApp([]string{"--cursor", "--quiet"})
	res := fixtureResult()
	res.Errors = []string{"trae: 登录态在加密存储中，没有可读的 JWT 文件"}
	app.Scan = func(adapter.Home) scan.Result { return res }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "加密存储") {
		t.Fatalf("cursor slice should not foot-note Trae login:\n%s", s)
	}
}

func TestRunDefaultPrintsP0Table(t *testing.T) {
	app, out, errb := testApp(nil)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d err=%s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"总用量", "命中率", "最长连烧", "当前连烧", "请求", "用户回合", "11.68 M", "85.2%", "占比", "近7日"} {
		if !strings.Contains(s, want) {
			t.Errorf("missing %q in\n%s", want, s)
		}
	}
	if strings.Contains(s, "消耗") {
		t.Fatal("消耗 watermark")
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunHelpAndVersion(t *testing.T) {
	app, out, _ := testApp([]string{"--help"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(out.String(), "USAGE") || !strings.Contains(out.String(), "EXIT CODES") {
		t.Fatalf("help:\n%s", out.String())
	}
	app, out, _ = testApp([]string{"--version"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if strings.TrimSpace(out.String()) != "wheretoken test" {
		t.Fatalf("version=%q", out.String())
	}
}

func TestRunUnknownCommandExitUsage(t *testing.T) {
	app, _, errb := testApp([]string{"explode"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errb.String(), "explode") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunUnknownModelExitUsage(t *testing.T) {
	app, _, errb := testApp([]string{"--model=nope-model"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "nope-model") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunJSONOfflineNote(t *testing.T) {
	app, out, errb := testApp([]string{"--json", "--offline"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, `"schema": 1`) {
		t.Fatalf("not schema 1:\n%s", s)
	}
	if !strings.Contains(s, "offline") || !strings.Contains(s, "本机账本") {
		t.Fatalf("JSON must keep the offline note scripts can read:\n%s", s)
	}
}

func TestRunJSON(t *testing.T) {
	app, out, errb := testApp([]string{"--json"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, `"total_m": "11.68 M"`) {
		t.Fatalf("%s", s)
	}
	if strings.Contains(s, "┌") {
		t.Fatal("json must not be a table")
	}
	var m map[string]any
	if err := json.Unmarshal(out.Bytes(), &m); err != nil {
		t.Fatal(err)
	}
	assertCLICommunityAgainstSchema(t, m)
	if strings.Contains(s, `"rank": 0`) || strings.Contains(s, `"rank":0`) || strings.Contains(s, `"#0`) {
		t.Fatalf("unavailable rank must not be 0:\n%s", s)
	}
}

func TestRunJSONCommunityMatchesPublishedSchema(t *testing.T) {
	for _, args := range [][]string{
		{"--json"},
		{"--json", "--offline"},
		{"--json", "--no-community"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			app, out, errb := testApp(args)
			if code := app.Run(); code != ExitOK {
				t.Fatalf("code=%d %s", code, errb.String())
			}
			var m map[string]any
			if err := json.Unmarshal(out.Bytes(), &m); err != nil {
				t.Fatal(err)
			}
			assertCLICommunityAgainstSchema(t, m)
		})
	}
}

func TestRunJSONIgnoresASCIIAndNeverANSI(t *testing.T) {
	app, out, errb := testApp([]string{"--json", "--ascii"})
	app.StdoutTTY = true
	app.LookupEnv = func(k string) string {
		if k == "FORCE_COLOR" {
			return "1"
		}
		return ""
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "+--") || strings.Contains(s, "┌") || strings.Contains(s, "\x1b") {
		t.Fatalf("json must stay machine-readable:\n%s", s)
	}
}

func TestRunRespectsWHERETOKEN_HOME(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	app, out, errb := testApp(nil)
	app.Scan = nil
	app.Home = nil
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_HOME" {
			return dir
		}
		return ""
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "占比") {
		t.Fatalf("scanned the real home instead of WHERETOKEN_HOME:\n%s", s)
	}
	if !strings.Contains(s, "0.00 M") {
		t.Fatalf("expected empty fake home, got:\n%s", s)
	}
}

func TestRunScanJSONRedactsJWT(t *testing.T) {
	jwt := "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9.eyJzdWIiOiIxMjM0In0.abc"
	app, out, errb := testApp([]string{"scan"})
	res := fixtureResult()
	res.Errors = []string{"trae: bearer " + jwt}
	app.Scan = func(adapter.Home) scan.Result { return res }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "eyJ") || strings.Contains(s, jwt) {
		t.Fatalf("scan JSON leaked JWT:\n%s", s)
	}
}

func TestRunTodayCursorCombo(t *testing.T) {
	app, out, errb := testApp([]string{"--today", "--cursor"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Cursor") || !strings.Contains(s, "今天") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, "0.00 M") {
		t.Fatalf("cursor today should be zero in fixture:\n%s", s)
	}
}

func TestRunTodayKimiCombo(t *testing.T) {
	app, out, errb := testApp([]string{"--today", "--kimi", "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Kimi") || !strings.Contains(s, "今天") {
		t.Fatalf("%s", s)
	}
	if !strings.Contains(s, "1.03 M") {
		t.Fatalf("today kimi should be today's 1.03 M:\n%s", s)
	}
	if strings.Contains(s, "Claude") || strings.Contains(s, "11.68") || strings.Contains(s, "10.65") {
		t.Fatalf("today --kimi mixed all-time/other tools:\n%s", s)
	}
}

func TestRunOfflineEnvAddsTableBanner(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, out, errb := testApp([]string{"--ascii", "--quiet"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_OFFLINE" {
			return "1"
		}
		return ""
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "offline") || !strings.Contains(out.String(), "本机账本") {
		t.Fatalf("WHERETOKEN_OFFLINE=1 must banner the table:\n%s", out.String())
	}
}

func TestRunEmptyHomeClaudeSliceExplains(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, out, errb := testApp([]string{"--claude", "--ascii", "--quiet"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Claude Code") || !strings.Contains(out.String(), "没有找到账本") {
		t.Fatalf("--claude on empty home:\n%s", out.String())
	}
}

func TestRunEmptyHomeModelK3IsOk(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, out, errb := testApp([]string{"--model=k3", "--ascii", "--quiet"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "k3") {
		t.Fatalf("empty home --model=k3:\n%s", out.String())
	}
}

func TestRunEmptyHomeExplainsMissingLedgers(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, out, errb := testApp([]string{"--ascii", "--quiet"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "panic") || strings.Contains(errb.String(), "panic") {
		t.Fatalf("empty home crashed:\n%s\n%s", s, errb.String())
	}
	if !strings.Contains(s, "0.00 M") {
		t.Fatalf("expected zeros:\n%s", s)
	}
	if !strings.Contains(s, "本机没有找到账本") {
		t.Fatalf("empty home should say so:\n%s", s)
	}
}

func TestRunUnknownFlagExitUsage(t *testing.T) {
	app, _, errb := testApp([]string{"--nope"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if strings.Contains(errb.String(), "flag provided but not defined") {
		t.Fatalf("Go flag jargon: %s", errb.String())
	}
	if !strings.Contains(errb.String(), "unknown flag") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunScanJSONStillFullSummary(t *testing.T) {
	app, out, errb := testApp([]string{"scan", "--json"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), `"by_source"`) || !strings.Contains(out.String(), `"calendar"`) {
		t.Fatalf("expected observatory JSON:\n%s", out.String())
	}
}

func TestServeStartedMessageTellsStrangerAboutRefresh(t *testing.T) {
	msg := ServeStartedMessage("127.0.0.1:8787")
	if !strings.Contains(msg, "http://127.0.0.1:8787") {
		t.Fatalf("missing URL:\n%s", msg)
	}
	if !strings.Contains(msg, "刷新") {
		t.Fatalf("stranger must be told 刷新 rescans:\n%s", msg)
	}
	if !strings.Contains(msg, "重载") && !strings.Contains(msg, "F5") {
		t.Fatalf("must say reloading the tab is not a rescan:\n%s", msg)
	}
}

func TestRunServePrintsRefreshHint(t *testing.T) {
	app, _, errb := testApp([]string{"serve", "--port", "8791"})
	app.Serve = func(addr string, home adapter.Home, _ bool) error {
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := errb.String()
	if !strings.Contains(s, "http://127.0.0.1:8791") {
		t.Fatalf("stderr missing URL:\n%s", s)
	}
	if !strings.Contains(s, "刷新") {
		t.Fatalf("stderr missing 刷新 hint:\n%s", s)
	}
}

func TestRunServeDoesNotScanTable(t *testing.T) {
	app, out, errb := testApp([]string{"serve", "--port", "8787"})
	called := false
	app.Serve = func(addr string, home adapter.Home, _ bool) error {
		called = true
		if !strings.HasPrefix(addr, "127.0.0.1:") {
			t.Fatalf("addr=%s", addr)
		}
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !called {
		t.Fatal("serve not called")
	}
	if strings.Contains(out.String(), "总用量") {
		t.Fatal("serve should not print the table")
	}
}

func TestRunServePassesOfflineFlag(t *testing.T) {
	app, _, errb := testApp([]string{"serve", "--offline", "--port", "8790"})
	var got *bool
	app.Serve = func(addr string, home adapter.Home, offline bool) error {
		got = &offline
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if got == nil || !*got {
		t.Fatal("serve must receive offline=true so dashboard 刷新 skips Cursor/Trae APIs")
	}
}

func TestRunServeHonorsOfflineEnv(t *testing.T) {
	app, _, errb := testApp([]string{"serve", "--port", "8790"})
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_OFFLINE" {
			return "1"
		}
		return ""
	}
	offline := false
	app.Serve = func(addr string, home adapter.Home, off bool) error {
		offline = off
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !offline {
		t.Fatal("WHERETOKEN_OFFLINE=1 must make serve skip cloud APIs")
	}
}

func TestRunServeFailureIsExitFail(t *testing.T) {
	app, _, errb := testApp([]string{"serve", "--port", "8787"})
	app.Serve = func(addr string, home adapter.Home, _ bool) error {
		return errors.New("bind 127.0.0.1:8787: address already in use")
	}
	if code := app.Run(); code != ExitFail {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "already in use") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunDegradedTraeDoesNotCrash(t *testing.T) {
	app, out, errb := testApp(nil)
	app.Scan = func(adapter.Home) scan.Result {
		r := fixtureResult()
		r.Errors = []string{"trae: 登录态在加密存储中，没有可读的 JWT 文件"}
		return r
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Trae") {
		t.Fatalf("%s", out.String())
	}
	if strings.Contains(out.String(), "eyJ") {
		t.Fatal("jwt")
	}
}

func TestRunNO_COLORNoEscape(t *testing.T) {
	app, out, _ := testApp(nil)
	app.StdoutTTY = true
	app.LookupEnv = func(k string) string {
		if k == "NO_COLOR" {
			return "1"
		}
		return ""
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if strings.Contains(out.String(), "\x1b") {
		t.Fatal("ANSI despite NO_COLOR")
	}
}

func TestHelpTextMentionsPrivacyAndInstall(t *testing.T) {
	h := HelpText()
	if strings.Contains(h, "消耗") {
		t.Fatal("help must not watermark 消耗")
	}
	for _, want := range []string{"go install", "curl -fsSL", "install.sh", "install.ps1", "install.cmd", "JWT", "127.0.0.1", "EXIT CODES", "--tool", "--today", "EXAMPLES", "NO_COLOR", "WHERETOKEN_HOME", "--quiet", "--width", "truncating names", "--offline", "FORCE_COLOR", "--today --cursor", "--today --kimi", "--today --grok", "schema 1", "per-tool", "--model=k3", "cli-json.schema.json", "[flags] sources", "--version", "--port", "./Formula/wheretoken.rb", "unsigned", "brew tap rainhuang0220/wheretoken", "刷新", "xai", "CLI table", "observatory JSON", "not schema 1", "brew --HEAD", "scan is a different dump", "[flags] update", "[flags] uninstall", "community"} {
		if !strings.Contains(h, want) {
			t.Errorf("help missing %q", want)
		}
	}
	if strings.Contains(h, "GOPATH") {
		t.Fatal("help must not lecture GOPATH")
	}
	curl := strings.Index(h, "curl -fsSL")
	goInst := strings.Index(h, "go install")
	if curl < 0 || goInst < 0 || curl > goInst {
		t.Fatal("help INSTALL should lead with curl | bash, then go install")
	}
}

func TestHelpDoesNotAdvertiseUnpublishedNpm(t *testing.T) {
	h := HelpText()
	if strings.Contains(h, "npm install") || strings.Contains(h, "npx wheretoken") {
		t.Fatal("wheretoken is not on the npm registry; help must not list npm as an install path")
	}
}

func TestRunSourcesListsRoots(t *testing.T) {
	app, out, errb := testApp([]string{"sources"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Roots: []adapter.SourceRoot{{ID: "kimi", Path: "/tmp/fake/.kimi-code"}}}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "kimi") || !strings.Contains(out.String(), "/tmp/fake") {
		t.Fatalf("%s", out.String())
	}
	if strings.Contains(errb.String(), "没有找到") {
		t.Fatalf("hint leaked when roots exist: %s", errb.String())
	}
}

func TestRunSourcesEmptyHintsOnStderr(t *testing.T) {
	app, out, errb := testApp([]string{"sources"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if out.Len() != 0 {
		t.Fatalf("empty sources must keep stdout empty for scripts: %q", out.String())
	}
	if !strings.Contains(errb.String(), "没有找到本机账本") {
		t.Fatalf("stderr=%q", errb.String())
	}
}

func TestRunSinceJSONUsesLocalWindow(t *testing.T) {
	app, out, errb := testApp([]string{"--since", "1d", "--json"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, `"period": "今天 2026-08-16"`) && !strings.Contains(s, `"period": "近 1 天"`) {
		t.Fatalf("period:\n%s", s)
	}
	if !strings.Contains(s, `"total": 1580000`) {
		t.Fatalf("1d should keep 16 Aug only:\n%s", s)
	}
}

func TestRunRebuildWipesThenReports(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"rebuild", "--home", dir, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "whereToken") {
		t.Fatalf("rebuild should print the table:\n%s", out.String())
	}
}

func TestRunQuietSuppressesProgress(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, _, errb := testApp([]string{"--quiet"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	app.StderrTTY = true
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if strings.Contains(errb.String(), "正在读") {
		t.Fatalf("quiet still printed progress: %s", errb.String())
	}
}

func TestRunProgressOnStderrWhenTTY(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, out, errb := testApp(nil)
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	app.StderrTTY = true
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "正在读") {
		t.Fatalf("stderr=%s", errb.String())
	}
	if !strings.Contains(errb.String(), "▄██████▄") && !strings.Contains(errb.String(), "+------+") {
		t.Fatalf("expected clawd slab on stderr:\n%s", errb.String())
	}
	if !strings.Contains(errb.String(), "\r") {
		t.Fatal("progress should redraw in place")
	}
	if strings.Contains(out.String(), "正在读") {
		t.Fatal("progress leaked to stdout")
	}
}

func TestRunASCIIMascotOnProgress(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, _, errb := testApp([]string{"--ascii"})
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	app.StderrTTY = true
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "+------+") {
		t.Fatalf("ascii slab:\n%s", errb.String())
	}
	if strings.ContainsAny(errb.String(), "▄█▀▌") {
		t.Fatal("ascii slab leaked unicode")
	}
}

func TestRunSilentProgressWhenNotTTY(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	app, _, errb := testApp(nil)
	app.Scan = nil
	app.Home = testhome.New(t.TempDir())
	app.StderrTTY = false
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d", code)
	}
	if strings.Contains(errb.String(), "正在读") {
		t.Fatalf("progress on pipe: %s", errb.String())
	}
}

func TestResolveWidthFlagAndCOLUMNS(t *testing.T) {
	if got := resolveWidth(80, func(string) string { return "40" }, func() int { return 20 }); got != 80 {
		t.Fatalf("flag should win: %d", got)
	}
	if got := resolveWidth(0, func(k string) string {
		if k == "COLUMNS" {
			return "72"
		}
		return ""
	}, func() int { return 20 }); got != 72 {
		t.Fatalf("COLUMNS: %d", got)
	}
	if got := resolveWidth(0, func(string) string { return "" }, func() int { return 40 }); got != 40 {
		t.Fatalf("tty size: %d", got)
	}
	if got := resolveWidth(0, func(string) string { return "" }, nil); got != 0 {
		t.Fatalf("empty: %d", got)
	}
	if got := resolveWidth(0, func(string) string { return "nope" }, nil); got != 0 {
		t.Fatalf("garbage COLUMNS: %d", got)
	}
	if got := resolveWidth(0, func(string) string { return "-40" }, nil); got != 0 {
		t.Fatalf("negative COLUMNS: %d", got)
	}
}

func TestCompletionShellThenQuiet(t *testing.T) {
	app, out, errb := testApp([]string{"completion", "zsh", "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "_arguments") {
		t.Fatalf("stdout=%s", out.String())
	}
}

func TestCompletionRequiresShell(t *testing.T) {
	app, _, errb := testApp([]string{"completion"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errb.String(), "bash") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestCompletionDoctorOffersNoCommunity(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		s, err := Completion(sh)
		if err != nil {
			t.Fatal(err)
		}
		switch sh {
		case "bash":
			if !strings.Contains(s, `doctor) opts="--quiet -q --offline --home --help --ascii --no-color --no-community"`) {
				t.Fatalf("bash doctor must offer --no-community")
			}
		case "zsh":
			i, j := strings.Index(s, "    doctor)"), strings.Index(s, "    rebuild)")
			if i < 0 || j <= i {
				t.Fatal("zsh doctor/rebuild branches")
			}
			if !strings.Contains(s[i:j], "--no-community") {
				t.Fatalf("zsh doctor must offer --no-community:\n%s", s[i:j])
			}
		case "fish":
			if !strings.Contains(s, `-l no-community`) {
				t.Fatal("fish missing --no-community")
			}
			if strings.Contains(s, `scan sources doctor community completion" -l no-community`) {
				t.Fatal("fish must not hide --no-community after doctor")
			}
		case "powershell":
			if !strings.Contains(s, `'doctor' { @('--quiet','--offline','--home','--help','--ascii','--no-color','--no-community') }`) {
				t.Fatalf("powershell doctor must offer --no-community")
			}
		}
	}
}

func TestCompletionScanOmitsTableFilters(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		s, err := Completion(sh)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.Contains(s, "scan") {
			t.Fatalf("%s missing scan branch", sh)
		}
		switch sh {
		case "bash":
			if !strings.Contains(s, `scan) opts="--json --quiet -q --offline --home --ascii --no-color --help"`) {
				t.Fatalf("bash scan opts still offer table flags:\n%s", s)
			}
		case "zsh":
			if strings.Contains(s, "scan)\n      _arguments") && strings.Contains(s[strings.Index(s, "scan)"):strings.Index(s, "serve)")], "--today") {
				t.Fatalf("zsh scan branch still offers --today")
			}
		case "fish":
			if !strings.Contains(s, `__fish_seen_subcommand_from scan`) || !strings.Contains(s, `-l today`) {
				t.Fatalf("fish should hide --today after scan")
			}
		case "powershell":
			if !strings.Contains(s, `'scan' { @('--json'`) {
				t.Fatalf("powershell scan branch missing")
			}
		}
	}
}

func TestCompletionShells(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		s, err := Completion(sh)
		if err != nil || !strings.Contains(s, "wheretoken") {
			t.Fatalf("%s: %v %q", sh, err, s)
		}
	}
	_, err := Completion("cmd")
	if err == nil || !IsUsage(err) {
		t.Fatalf("err=%v", err)
	}
}

func TestRunReportPrintsPublicSiteFooter(t *testing.T) {
	app, out, errb := testApp(nil)
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Web: https://rainhuang0220.github.io/whereToken/\n") {
		t.Fatalf("human report must carry the public site footer:\n%s", s)
	}
	if !strings.HasSuffix(strings.TrimRight(s, "\n"), "Web: https://rainhuang0220.github.io/whereToken/") {
		t.Fatalf("footer should be the last line:\n%s", s)
	}
}

func TestRunJSONOmitsPublicSiteFooter(t *testing.T) {
	app, out, errb := testApp([]string{"--json"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "rainhuang0220.github.io") || strings.Contains(s, "Web:") {
		t.Fatalf("schema 1 JSON must stay clean of the footer:\n%s", s)
	}
}

func TestServeStartedMessageShowsLocalAndPublic(t *testing.T) {
	msg := ServeStartedMessage("127.0.0.1:8787")
	for _, want := range []string{
		"http://127.0.0.1:8787",
		"Public: https://rainhuang0220.github.io/whereToken/",
		"公网仅展示公开/演示数据，本地账本不会因此上传。",
	} {
		if !strings.Contains(msg, want) {
			t.Fatalf("serve message missing %q:\n%s", want, msg)
		}
	}
	// The local line stays first and untouched.
	if !strings.HasPrefix(msg, "http://127.0.0.1:8787\n") {
		t.Fatalf("local URL line must lead:\n%s", msg)
	}
}
