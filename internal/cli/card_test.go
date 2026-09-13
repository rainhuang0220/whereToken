package cli

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestRunCardWritesSVGAndStdout(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "wall.svg")
	app, stdout, stderr := testApp([]string{"card", outPath, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.HasPrefix(strings.TrimSpace(stdout.String()), "wrote ") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	if !strings.Contains(stdout.String(), outPath) && !strings.Contains(stdout.String(), filepath.Base(outPath)) {
		t.Fatalf("stdout missing path: %q", stdout.String())
	}
	if stderr.Len() != 0 {
		t.Fatalf("stderr=%s", stderr.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, `viewBox="0 0 800 576"`) {
		t.Fatalf("not an SVG card:\n%s", s[:min(200, len(s))])
	}
	if strings.Contains(s, outPath) {
		t.Fatal("output path leaked into SVG")
	}
}

func TestRunCardFlagsBeforeCommand(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "a.svg")
	app, _, stderr := testApp([]string{"--offline", "--quiet", "card", outPath})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
}

func TestRunCardMissingPathAndBadExtension(t *testing.T) {
	app, _, errb := testApp([]string{"card"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errb.String(), ".svg") {
		t.Fatalf("stderr=%s", errb.String())
	}
	app, _, errb = testApp([]string{"card", "out.png"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
	if !strings.Contains(errb.String(), ".svg") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunCardIncompatibleFlags(t *testing.T) {
	app, _, _ := testApp([]string{"card", "out.svg", "--json"})
	if code := app.Run(); code != ExitUsage {
		t.Fatalf("code=%d", code)
	}
}

func TestRunCardEmptyLedgerExitOK(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "empty.svg")
	app, _, stderr := testApp([]string{"card", outPath, "--quiet"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Summary: metric.AggregateAt(nil, nil, app.Now(), app.Loc)}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "NO LOCAL USAGE AVAILABLE") {
		t.Fatalf("empty card:\n%s", s)
	}
	if strings.Contains(s, `data-state="empty"`) {
		t.Fatal("unavailable painted empty zeros")
	}
	if strings.Contains(s, "$0") {
		t.Fatal("$0 on empty card")
	}
}

func TestRunCardReplacesExisting(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "wall.svg")
	if err := os.WriteFile(outPath, []byte("OLD_SVG_BYTES"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, _, stderr := testApp([]string{"card", outPath, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(raw, []byte("OLD_SVG_BYTES")) {
		t.Fatal("old contents remained")
	}
	if !bytes.Contains(raw, []byte("<svg")) {
		t.Fatal("new SVG missing")
	}
	if _, err := os.Stat(outPath + ".old"); !os.IsNotExist(err) {
		t.Fatalf("leftover backup: %v", err)
	}
}

func TestReplaceCardFileKeepsOldOnRenameFailure(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "wall.svg")
	if err := os.WriteFile(dest, []byte("KEEP-ME"), 0o644); err != nil {
		t.Fatal(err)
	}
	old := renameFile
	t.Cleanup(func() { renameFile = old })
	renameFile = func(from, to string) error {
		if strings.Contains(filepath.Base(from), ".wheretoken-card-") {
			return os.ErrPermission
		}
		return os.Rename(from, to)
	}
	err := replaceCardFile(dest, []byte("<svg>new</svg>"))
	if err == nil {
		t.Fatal("expected failure")
	}
	got, readErr := os.ReadFile(dest)
	if readErr != nil {
		t.Fatal(readErr)
	}
	if string(got) != "KEEP-ME" {
		t.Fatalf("old file lost: %q", got)
	}
}

func TestRunCardParentMissingAndDirectory(t *testing.T) {
	dir := t.TempDir()
	app, stdout, errb := testApp([]string{"card", filepath.Join(dir, "nope", "out.svg"), "--quiet"})
	if code := app.Run(); code != ExitFail {
		t.Fatalf("missing parent code=%d stderr=%s", code, errb.String())
	}
	if strings.Contains(stdout.String(), "wrote") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	subdir := filepath.Join(dir, "as-dir.svg")
	if err := os.Mkdir(subdir, 0o755); err != nil {
		t.Fatal(err)
	}
	app, _, errb = testApp([]string{"card", subdir, "--quiet"})
	if code := app.Run(); code != ExitFail {
		t.Fatalf("dir code=%d stderr=%s", code, errb.String())
	}
	if !strings.Contains(errb.String(), "directory") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunCardUnwritablePath(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("unix permissions")
	}
	dir := t.TempDir()
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { os.Chmod(dir, 0o755) })
	app, _, errb := testApp([]string{"card", filepath.Join(dir, "out.svg"), "--quiet"})
	if code := app.Run(); code != ExitFail {
		t.Fatalf("code=%d stderr=%s", code, errb.String())
	}
}

func TestRunCardHomeEmptyIsUnavailableSVG(t *testing.T) {
	home := t.TempDir()
	outPath := filepath.Join(t.TempDir(), "empty.svg")
	app, stdout, stderr := testApp([]string{"--home", home, "--quiet", "card", outPath})
	app.Scan = nil
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if !strings.Contains(s, "NO LOCAL USAGE AVAILABLE") {
		t.Fatalf("%s", s)
	}
	if strings.Contains(s, `data-state="empty"`) {
		t.Fatal("empty home painted measured zeros")
	}
}

func TestRunCardDoesNotContactCommunity(t *testing.T) {
	var hits atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		hits.Add(1)
	}))
	t.Cleanup(srv.Close)
	dir := t.TempDir()
	home := testhome.New(dir)
	cfg := community.ConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(cfg), 0o755); err != nil {
		t.Fatal(err)
	}
	outPath := filepath.Join(dir, "wall.svg")
	app, _, stderr := testApp([]string{"card", outPath, "--quiet"})
	app.Home = home
	app.LookupEnv = func(k string) string {
		if k == "WHERETOKEN_COMMUNITY_URL" {
			return srv.URL
		}
		return ""
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	if hits.Load() != 0 {
		t.Fatalf("community contacted %d times", hits.Load())
	}
}

func TestRunCardPrivacySentinelsAbsentFromSVG(t *testing.T) {
	dir := t.TempDir()
	outPath := filepath.Join(dir, "wall.svg")
	loc := shanghai()
	e := event.UsageEvent{
		Source:     "claude",
		Vendor:     "anthropic",
		Model:      "claude-opus-4.6-poison",
		Workspace:  "/Users/alice/src/secret-repo",
		SessionID:  "sess_LIVE_SECRET_99",
		RequestID:  "req_LIVE_SECRET_99",
		SourceRoot: "/Users/alice/.claude",
		Timestamp:  time.Date(2026, 8, 16, 10, 0, 0, 0, loc),
		Miss:       100,
		Output:     5,
		Quality:    event.QualityAuthoritative,
	}
	app, _, stderr := testApp([]string{"card", outPath, "--quiet"})
	app.Scan = func(adapter.Home) scan.Result {
		evs := []event.UsageEvent{e}
		return scan.Result{
			Summary: metric.AggregateAt(evs, nil, app.Now(), app.Loc),
			Events:  evs,
			Errors:  []string{"claude: /Users/alice/.claude failed JWT eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9"},
			Roots:   []adapter.SourceRoot{{ID: "claude", Path: "/Users/alice/.claude"}},
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, stderr.String())
	}
	raw, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	for _, poison := range []string{
		"/Users/alice", "secret-repo", "sess_LIVE_SECRET_99", "req_LIVE_SECRET_99",
		"claude-opus-4.6-poison", "eyJhbGciOiJIUzI1NiIsInR5cCI6IkpXVCJ9",
	} {
		if strings.Contains(s, poison) {
			t.Fatalf("SVG leaked %q", poison)
		}
	}
}
