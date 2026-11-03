package ledger

import (
	"errors"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestCursorLinkRollsBackAndStaysIdempotent(t *testing.T) {
	lg := openTest(t)
	day := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-21",
		Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
		Miss:      539187, CacheRead: 5675130, CacheCreate: 77078, Output: 55585,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), []event.UsageEvent{day}); err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	lg.beforeMetaCommit = func() error { return errors.New("power loss") }
	if err := lg.CommitCursorLink(CursorLink{Subject: "test-user", Decision: "confirmed", At: when}); err == nil {
		t.Fatal("expected rollback")
	}
	lg.beforeMetaCommit = nil
	for _, key := range []string{cursorSubjectKey, cursorDecisionKey, cursorAuditSourceKey, cursorAuditAtKey, cursorBaselineKey, cursorDecisionSubjectKey} {
		got, err := lg.Meta(key)
		if err != nil || got != "" {
			t.Fatalf("%s=%q err=%v", key, got, err)
		}
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || len(chained) != 1 || chained[0].Miss != 539187 {
		t.Fatalf("history changed ok=%v err=%v %+v", ok, err, chained)
	}

	if err := lg.CommitCursorLink(CursorLink{Subject: "test-user", Decision: "confirmed", At: when}); err != nil {
		t.Fatal(err)
	}
	if err := lg.CommitCursorLink(CursorLink{Subject: "other-user", Decision: "confirmed", At: when.Add(time.Hour)}); !errors.Is(err, ErrCursorAccountMismatch) {
		t.Fatalf("err=%v", err)
	}
	if got, _ := lg.Meta(cursorSubjectKey); got != "test-user" {
		t.Fatalf("subject=%q", got)
	}
}
