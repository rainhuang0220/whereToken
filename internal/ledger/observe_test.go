package ledger

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func apiEvent(day int, miss int64) event.UsageEvent {
	return event.UsageEvent{
		Source: "cursor", RequestID: "api", Model: "m",
		Timestamp: time.Date(2026, 1, day, 3, 0, 0, 0, time.UTC),
		Miss:      miss, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
}

func TestMissingObservationIsNotZero(t *testing.T) {
	lg := openTest(t)
	events, ok, err := lg.LatestRetainedEvents("cursor")
	if err != nil || ok || len(events) != 0 {
		t.Fatalf("missing observation events=%d ok=%v err=%v", len(events), ok, err)
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Time{}, nil); err == nil {
		t.Fatal("rejected observation was stored")
	}
}

func TestRepeatedObservationIsOneRowAndLatestIsNotFieldMax(t *testing.T) {
	lg := openTest(t)
	when := time.Date(2026, 10, 20, 0, 0, 0, 0, time.UTC)
	wide := []event.UsageEvent{
		{Source: "cursor", Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), Miss: 100, CacheRead: 5},
		{Source: "cursor", Timestamp: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), Miss: 1, Output: 7},
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", when, wide); err != nil {
		t.Fatal(err)
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", when, wide); err != nil {
		t.Fatal(err)
	}
	later := when.Add(time.Hour)
	revised := []event.UsageEvent{
		{Source: "cursor", Timestamp: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), Miss: 3, Output: 1},
	}
	if err := lg.SaveObservation("cursor", "api", "complete", later, revised); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("observations=%d %v", n, err)
	}
	got, ok, err := lg.LatestRetainedEvents("cursor")
	if err != nil || !ok {
		t.Fatalf("latest ok=%v err=%v", ok, err)
	}
	if tokensOf(got) != 4 {
		t.Fatalf("latest spliced or kept the older total: %d", tokensOf(got))
	}
	if got[0].Derivation != event.DeriveRetained || got[0].Quality != event.QualityDegraded || !got[0].SkipRequest {
		t.Fatalf("reuse was marked fresh: %+v", got[0])
	}
	st, _ := lg.Status()
	if st.Facts != 0 || st.Tokens != 0 {
		t.Fatalf("observation became a fact %+v", st)
	}
}

func TestNarrowerWindowKeepsEarlierDaysAndUsesTheNewOverlap(t *testing.T) {
	lg := openTest(t)
	previous := []event.UsageEvent{
		{Source: "cursor", Timestamp: time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC), Miss: 100, CacheRead: 1},
		{Source: "cursor", Timestamp: time.Date(2026, 1, 2, 12, 0, 0, 0, time.UTC), Miss: 50},
	}
	if err := lg.SaveObservation("cursor", "api", "complete", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), previous); err != nil {
		t.Fatal(err)
	}
	fresh := apiEvent(2, 40)
	if err := lg.NoteAPI([]event.UsageEvent{fresh}); err != nil {
		t.Fatal(err)
	}
	got, err := lg.SupplementNarrowWindow([]event.UsageEvent{fresh})
	if err != nil {
		t.Fatal(err)
	}
	if tokensOf(got) != 141 {
		t.Fatalf("narrow window total=%d, want earlier 101 plus the new day 40", tokensOf(got))
	}
	var overlap int64
	for _, e := range got {
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-01-02" {
			overlap += e.Miss + e.CacheRead + e.CacheCreate + e.Output
		}
	}
	if overlap != 40 {
		t.Fatalf("overlap used a spliced day total %d", overlap)
	}
}

func TestRetainedReuseDoesNotAddLocalFacts(t *testing.T) {
	lg := openTest(t)
	local := ev("cursor", "s", "bubble-1", 100)
	if err := lg.CommitUsage([]event.UsageEvent{local}); err != nil {
		t.Fatal(err)
	}
	retained := retainedEvent("cursor", "2026-01-02", 7, 0, 0, 1)
	extra, err := lg.Unseen([]event.UsageEvent{retained})
	if err != nil || len(extra) != 0 {
		t.Fatalf("retained scan added local facts %d %v", len(extra), err)
	}
	if err := lg.CommitUsage([]event.UsageEvent{retained}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Facts != 1 || st.Tokens != 101 {
		t.Fatalf("retained row changed facts %+v", st)
	}
}
