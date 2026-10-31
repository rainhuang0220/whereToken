package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestRunSealedCursorReportShowsUnavailable(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, loc)
	evs := []event.UsageEvent{{
		Source: "cursor", Vendor: "unknown", Model: "composer", RequestID: "sess:a",
		Timestamp: now, Quality: event.QualityDegraded, Derivation: event.DeriveRaw,
	}}
	turns := []event.TurnEvent{{Source: "cursor", Timestamp: now}}
	app, out, errb := testApp([]string{"--cursor", "--quiet", "--ascii"})
	app.Now = func() time.Time { return now }
	app.Loc = loc
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{
			Roots:   []adapter.SourceRoot{{ID: "cursor", Path: "state.vscdb"}},
			Events:  evs,
			Turns:   turns,
			Summary: metric.Aggregate(evs, turns),
			Errors:  []string{"cursor: 账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥"},
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	for _, want := range []string{"账号用量不可用", "不可用"} {
		if !strings.Contains(s, want) {
			t.Fatalf("missing %q:\n%s", want, s)
		}
	}
	for _, bad := range []string{"0.00 M", "钥匙串", "keychain", "需要已登录", "eyJ"} {
		if strings.Contains(s, bad) {
			t.Fatalf("report contains %q:\n%s", bad, s)
		}
	}
}

func TestRunDoctorSealedCursorDoesNotAskForKeychain(t *testing.T) {
	app, out, errb := testApp([]string{"doctor"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{
			Roots: []adapter.SourceRoot{{ID: "cursor", Path: "state.vscdb"}},
			Summary: metric.Summary{BySource: []metric.Slice{{
				ID: "cursor", Label: "Cursor", Requests: 2, Quality: event.QualityDegraded,
			}}},
			Errors: []string{"cursor: 账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥"},
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "Cursor") || !strings.Contains(s, "degraded") || !strings.Contains(s, "账号用量不可用") {
		t.Fatalf("doctor:\n%s", s)
	}
	if strings.Contains(strings.ToLower(s), "keychain") || strings.Contains(s, "钥匙串") || strings.Contains(s, "证书") {
		t.Fatalf("doctor asked for a custom trust setup:\n%s", s)
	}
}
