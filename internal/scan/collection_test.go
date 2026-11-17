package scan

import (
	"path/filepath"
	"testing"
	"time"

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
