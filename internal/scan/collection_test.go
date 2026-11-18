package scan

import (
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestCursorCoverageClosesOldOpenDaysAfterRecovery(t *testing.T) {
	lg, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	start := "2025-11-01"
	old := time.Date(2026, 11, 9, 10, 58, 44, 0, time.UTC)
	if err := lg.NoteAuditedWindow("cursor", ledger.WindowAudit{From: start, To: "2026-11-09", Validation: "open"}, old, nil); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 2, 32, 0, 0, time.UTC)
	stale := cursorCheck(now, now, lg, "cursor", "AUTH", false)
	if stale.Status != publicprofile.SourceAuthRequired || stale.CoverageThroughUTC != "2026-11-08" ||
		stale.LastObservationAt == nil || !stale.LastObservationAt.Equal(old) ||
		len(stale.GapsUTC) != 1 || stale.GapsUTC[0] != (publicprofile.UTCGap{From: "2026-11-09", To: "2026-11-16"}) {
		t.Fatalf("old open day credited as closed: %+v", stale)
	}
	empty := ledger.WindowAudit{From: "2026-11-09", To: "2026-11-16", Validation: "open"}
	for range 2 {
		if err := lg.NoteAuditedWindow("cursor", empty, now, nil); err != nil {
			t.Fatal(err)
		}
	}
	fresh := cursorCheck(now, now, lg, "cursor", "", true)
	if fresh.Status != publicprofile.SourceConfirmedZero || fresh.CoverageThroughUTC != "2026-11-15" || len(fresh.GapsUTC) != 0 {
		t.Fatalf("validated coverage was not restored: %+v", fresh)
	}
}

func TestKnownHistoricalGapUsesTheRegularSchedule(t *testing.T) {
	lg, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	if err := lg.NoteAuditedWindow("cursor", ledger.WindowAudit{
		From: "2026-11-01", To: "2026-11-18", Validation: "open",
	}, now, nil); err != nil {
		t.Fatal(err)
	}
	check := cursorCheck(now, now, lg, "cursor", "", true)
	state := &publicprofile.CollectionEvidence{Sources: []publicprofile.SourceCheck{check}}
	if check.Status != publicprofile.SourceCoverageGap || state.RetryCategory() != "" {
		t.Fatalf("a previously known gap spent the immediate retry budget: %+v", check)
	}
}

func TestAuditedZeroMeterRowsStayConfirmedZero(t *testing.T) {
	lg, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	at := now.Add(-time.Hour)
	audit := ledger.WindowAudit{
		From: "2025-11-12", To: "2026-11-18", Validation: "open",
		Declared: 1, RawRows: 1, Deduped: 1, Days: 1,
	}
	row := event.UsageEvent{
		Source: "cursor", RequestID: "fixture", Timestamp: at,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI,
	}
	if err := lg.NoteAuditedWindow("cursor", audit, now, []event.UsageEvent{row}); err != nil {
		t.Fatal(err)
	}
	check := cursorCheck(now, now, lg, "cursor", "", true)
	if check.Status != publicprofile.SourceConfirmedZero {
		t.Fatalf("audited zero meter was treated as %s", check.Status)
	}
}
