package cli

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestBoundCursorRetainedReportIsExplicitAndIdempotent(t *testing.T) {
	home := testhome.New(t.TempDir())
	path, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	snap := []event.UsageEvent{{Source: "cursor", RequestID: "old-day", Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC), Miss: 8_700_453_509, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true}}
	if err = lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), snap); err != nil {
		t.Fatal(err)
	}
	if err = lg.Close(); err != nil {
		t.Fatal(err)
	}
	createResult := func() scan.Result {
		return scan.Result{Errors: []string{"cursor: 账号用量不可用：ENDPOINT_UNAVAILABLE"}}
	}
	res := createResult()
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 0 {
		t.Fatal("unbound history became visible to a new account")
	}
	lg, err = ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = lg.SetMeta(cursor.AccountSubjectMeta, "auth0|user_test"); err != nil {
		t.Fatal(err)
	}
	if err = lg.Close(); err != nil {
		t.Fatal(err)
	}
	res = createResult()
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 0 {
		t.Fatal("subject alone must not auto-claim history")
	}
	lg, err = ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err = lg.SetMeta(cursor.HistoryDecisionMeta, cursor.HistoryConfirmed); err != nil {
		t.Fatal(err)
	}
	if err = lg.Close(); err != nil {
		t.Fatal(err)
	}
	res = createResult()
	res.Events = []event.UsageEvent{{Source: "cursor", RequestID: "session-1", Miss: 310_649_320, CacheRead: 50, Output: 10, Quality: event.QualityDegraded, Derivation: event.DeriveRaw}}
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 2 || res.Events[0].Miss != 0 || res.Events[0].CacheRead != 0 || res.Events[0].Output != 0 || res.Events[1].Miss != 8_700_453_509 {
		t.Fatalf("overlapping local cursor counters were double-counted: %+v", res.Events)
	}
	res = createResult()
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 1 || res.Events[0].Miss != 8_700_453_509 || res.Events[0].Derivation != event.DeriveRetained {
		t.Fatalf("verified retained event missing: %+v", res.Events)
	}
	if !strings.Contains(strings.Join(res.Errors, " "), "保留历史") {
		t.Fatalf("missing provenance note: %+v", res.Errors)
	}
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 1 {
		t.Fatal("duplicate historical events")
	}
	res = createResult()
	res.Events = []event.UsageEvent{{Source: "cursor", Timestamp: time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC), Miss: 1, Derivation: event.DeriveProviderAPI}}
	restoreBoundCursorHistoryForReport(home, &res)
	if len(res.Events) != 1 {
		t.Fatal("fresh API results double-counted historical state")
	}
}
