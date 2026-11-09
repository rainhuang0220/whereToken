package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestEnabledBackgroundRefreshMakesReportUseReadOnlyCache(t *testing.T) {
	home := testhome.New(t.TempDir())
	path := index.PathFor(home)
	st, err := index.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "not-a-real-agent.jsonl")
	if err := os.WriteFile(source, []byte("fixture\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 7, 10, 0, 0, 0, time.UTC)
	_, _, _, err = st.LoadOrReplay("claude", source, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		fi, err := f.Stat()
		if err != nil {
			return nil, nil, 0, err
		}
		e := event.UsageEvent{
			Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
			RequestID: "req-a", SessionID: "session-a", Timestamp: when,
			Miss: 100, Output: 10, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
		}
		turn := event.TurnEvent{Source: "claude", SessionID: "session-a", Timestamp: when}
		return []event.UsageEvent{e}, []event.TurnEvent{turn}, fi.Size(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := st.Close(); err != nil {
		t.Fatal(err)
	}
	lp, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(lp)
	if err != nil {
		t.Fatal(err)
	}
	lg.Close()
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	app, stdout, stderr := testApp([]string{"2026-11-07", "--no-community", "--json"})
	app.Scan = nil
	app.Home = home
	app.Now = func() time.Time { return time.Date(2026, 11, 9, 12, 0, 0, 0, time.UTC) }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d stderr=%s", code, stderr.String())
	}
	var got struct {
		Period    string   `json:"period"`
		Total     int64    `json:"total"`
		Notes     []string `json:"notes"`
		UserTurns int64    `json:"user_turns"`
	}
	if err := json.Unmarshal(stdout.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Total != 110 || got.UserTurns != 1 || got.Period != "2026-11-07 … 2026-11-07" {
		t.Fatalf("cached report wrong: %+v", got)
	}
	if !strings.Contains(strings.Join(got.Notes, "\n"), "缓存报告") {
		t.Fatalf("must explicitly mark cached—not live: %v", got.Notes)
	}
}

func TestCachedReportFailsClosedWithoutCacheOrRefresh(t *testing.T) {
	home := testhome.New(t.TempDir())
	app, _, _ := testApp(nil)
	app.Scan = nil
	app.Home = home
	if _, cached := app.cachedReport(home); cached {
		t.Fatal("disabled profile refresh must not activate cached mode")
	}
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	if _, cached := app.cachedReport(home); cached {
		t.Fatal("missing cache must fall back to normal scan, not fake zero")
	}
	if _, err := os.Stat(index.PathFor(home)); !os.IsNotExist(err) {
		t.Fatalf("cached report must not create an index: %v", err)
	}
}

func TestPricingUsageReadsSameCacheAsReport(t *testing.T) {
	home := testhome.New(t.TempDir())
	st, err := index.Open(index.PathFor(home))
	if err != nil {
		t.Fatal(err)
	}
	source := filepath.Join(t.TempDir(), "cached-only.jsonl")
	if err := os.WriteFile(source, []byte("fixture"), 0600); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = st.LoadOrReplay("claude", source, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return []event.UsageEvent{{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", Timestamp: time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC), Miss: 100, Output: 10, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw}}, nil, 7, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	st.Close()
	lp, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(lp)
	if err != nil {
		t.Fatal(err)
	}
	lg.Close()
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	app, out, errb := testApp([]string{"pricing", "--usage", "--json"})
	app.Scan = nil
	app.Home = home
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d err=%s", code, errb.String())
	}
	var got struct {
		Usage struct {
			Total struct {
				Total int64 `json:"total"`
			} `json:"total"`
			Notes []string `json:"notes"`
		} `json:"usage"`
	}
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	if got.Usage.Total.Total != 110 || !strings.Contains(strings.Join(got.Usage.Notes, " "), "缓存报告") {
		t.Fatalf("pricing bypassed report cache: %+v", got)
	}
}
