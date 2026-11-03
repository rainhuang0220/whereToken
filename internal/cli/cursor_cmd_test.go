package cli

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/scan"

	_ "modernc.org/sqlite"
)

// Synthetic subject {"sub":"test-user"}. Not a live session.
const cursorFixtureJWT = "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ0ZXN0LXVzZXIifQ.sig"

func TestCursorConnectStaysInsideHome(t *testing.T) {
	dir := t.TempDir()
	app, out, errb := testApp([]string{"--home", dir, "cursor", "connect"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: on") || !strings.Contains(out.String(), "session: missing") {
		t.Fatalf("out=%s err=%s", out.String(), errb.String())
	}
	if !strings.Contains(errb.String(), "操作系统保护的 Cursor 会话") {
		t.Fatalf("connect did not name the protected session:\n%s", errb.String())
	}
	if strings.Contains(out.String(), "eyJ") || strings.Contains(strings.ToLower(out.String()+errb.String()), "eyj") {
		t.Fatalf("status leaked a secret:\n%s", out.String())
	}
	app, out, errb = testApp([]string{"--home", dir, "cursor"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("status code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: on") {
		t.Fatalf("status=%s", out.String())
	}
	app, out, errb = testApp([]string{"--home", dir, "cursor", "disconnect"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("disconnect code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "cursor connect: off") || !strings.Contains(out.String(), "同意记录已删除") || !strings.Contains(out.String(), "系统里已经记住的允许还在") {
		t.Fatalf("disconnect=%s", out.String())
	}
	if strings.Contains(out.String(), "系统授权已撤销") || strings.Contains(out.String(), "钥匙串授权已删除") {
		t.Fatalf("disconnect claimed the operating system grant was removed:\n%s", out.String())
	}
	if _, err := Parse([]string{"cursor", "nope"}); err == nil {
		t.Fatal("unknown cursor action was accepted")
	}
}

func TestCursorConnectConfirmsUnboundHistoryBeforeCollecting(t *testing.T) {
	dir := t.TempDir()
	writeCursorFixtureDB(t, dir, cursorFixtureJWT)
	seedCursorFixtureHistory(t, dir)
	scanned := 0
	app, out, errb := testApp([]string{"--home", dir, "cursor", "connect"})
	app.Stdin = strings.NewReader("yes\n")
	app.Scan = func(adapter.Home) scan.Result {
		scanned++
		lg := openCLILedger(t, dir)
		defer lg.Close()
		if got, err := lg.Meta(cursor.AccountSubjectMeta); err != nil || got != "test-user" {
			t.Fatalf("collection started before the bind subject=%q err=%v", got, err)
		}
		return scan.Result{}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d out=%s err=%s", code, out.String(), errb.String())
	}
	text := out.String() + errb.String()
	for _, want := range []string{"没有足够证据把这份历史关联到当前账号", "已记录日数：2", "当前账号：test-user", "cursor connect: on", "session: jwt"} {
		if !strings.Contains(text, want) {
			t.Fatalf("missing %q\n%s", want, text)
		}
	}
	if strings.Contains(strings.ToLower(text), "eyj") || strings.Contains(text, "auth") {
		t.Fatalf("connect leaked material:\n%s", text)
	}
	if scanned != 1 {
		t.Fatalf("scans=%d", scanned)
	}

	app, out, errb = testApp([]string{"--home", dir, "cursor", "connect"})
	app.Stdin = strings.NewReader("no\n")
	app.Scan = func(adapter.Home) scan.Result {
		scanned++
		return scan.Result{}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("second code=%d err=%s", code, errb.String())
	}
	if strings.Contains(errb.String(), "输入 yes") {
		t.Fatalf("confirmed connect asked again:\n%s", errb.String())
	}
	if scanned != 2 {
		t.Fatalf("scans=%d", scanned)
	}
	lg := openCLILedger(t, dir)
	defer lg.Close()
	if got, _ := lg.Meta(cursor.HistoryDecisionMeta); got != cursor.HistoryConfirmed {
		t.Fatalf("decision=%q", got)
	}
}

func TestCursorConnectDeclineAndAmbiguousAnswerDoNotClaimHistory(t *testing.T) {
	dir := t.TempDir()
	writeCursorFixtureDB(t, dir, cursorFixtureJWT)
	seedCursorFixtureHistory(t, dir)
	scanned := 0
	app, _, errb := testApp([]string{"--home", dir, "cursor", "connect"})
	app.Stdin = strings.NewReader("y\n")
	app.Scan = func(adapter.Home) scan.Result {
		scanned++
		return scan.Result{}
	}
	if code := app.Run(); code == ExitOK {
		t.Fatal("ambiguous answer succeeded")
	}
	if scanned != 0 {
		t.Fatalf("ambiguous answer collected: %d", scanned)
	}
	if !strings.Contains(errb.String(), "没有记录这次选择") {
		t.Fatalf("err=%s", errb.String())
	}
	assertCLIHistoryUnbound(t, dir)

	app, _, errb = testApp([]string{"--home", dir, "cursor", "connect"})
	app.StdinInteractive = func() bool { return false }
	app.Scan = func(adapter.Home) scan.Result {
		scanned++
		return scan.Result{}
	}
	if code := app.Run(); code == ExitOK || scanned != 0 {
		t.Fatalf("non-terminal code=%d scans=%d", code, scanned)
	}
	if !strings.Contains(errb.String(), "请在终端里输入 yes 或 no") || !strings.Contains(errb.String(), "这次没有自动关联") {
		t.Fatalf("err=%s", errb.String())
	}
	assertCLIHistoryUnbound(t, dir)

	app, _, errb = testApp([]string{"--home", dir, "cursor", "connect"})
	app.Stdin = strings.NewReader("no\n")
	app.Scan = func(adapter.Home) scan.Result {
		scanned++
		return scan.Result{}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("decline code=%d err=%s", code, errb.String())
	}
	if scanned != 1 {
		t.Fatalf("scans=%d", scanned)
	}
	lg := openCLILedger(t, dir)
	defer lg.Close()
	if got, _ := lg.Meta(cursor.AccountSubjectMeta); got != "" {
		t.Fatalf("decline set subject %q", got)
	}
	if got, _ := lg.Meta(cursor.HistoryDecisionMeta); got != cursor.HistoryDeclined {
		t.Fatalf("decision=%q", got)
	}
}

func writeCursorFixtureDB(t *testing.T, home, token string) {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", filepath.Join(dir, "state.vscdb"))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	for _, key := range []string{"cursorAuth/accessToken", "cursorAuth/refreshToken"} {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, key, token); err != nil {
			t.Fatal(err)
		}
	}
}

func seedCursorFixtureHistory(t *testing.T, home string) {
	t.Helper()
	lg := openCLILedger(t, home)
	defer lg.Close()
	rows := []event.UsageEvent{
		{
			Source: "cursor", RequestID: "day-2025-10-21",
			Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
			Miss:      3, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
		{
			Source: "cursor", RequestID: "retained:2026-10-23",
			Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
			Miss:      5, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), rows); err != nil {
		t.Fatal(err)
	}
}

func openCLILedger(t *testing.T, home string) *ledger.Ledger {
	t.Helper()
	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	return lg
}

func assertCLIHistoryUnbound(t *testing.T, home string) {
	t.Helper()
	lg := openCLILedger(t, home)
	defer lg.Close()
	if got, err := lg.Meta(cursor.AccountSubjectMeta); err != nil || got != "" {
		t.Fatalf("subject=%q err=%v", got, err)
	}
	if got, err := lg.Meta(cursor.HistoryDecisionMeta); err != nil || got != "" {
		t.Fatalf("decision=%q err=%v", got, err)
	}
}
