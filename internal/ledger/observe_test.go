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

func TestChainedWindowsKeepEarlierDaysAndReplaceOverlap(t *testing.T) {
	lg := openTest(t)
	first := []event.UsageEvent{apiEvent(1, 100), apiEvent(2, 50), apiEvent(3, 7)}
	if err := lg.SaveObservation("cursor", "api", "complete", time.Date(2026, 10, 1, 0, 0, 0, 0, time.UTC), first); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-01-03", "2026-01-03", []event.UsageEvent{apiEvent(3, 9)}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-01-04", "2026-01-04", []event.UsageEvent{apiEvent(4, 4)}); err != nil {
		t.Fatal(err)
	}
	fresh := apiEvent(4, 4)
	got, err := lg.SupplementNarrowWindow([]event.UsageEvent{fresh})
	if err != nil {
		t.Fatal(err)
	}
	if tokensOf(got) != 163 {
		t.Fatalf("total=%d want 163", tokensOf(got))
	}
	var day3 int64
	for _, e := range got {
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-01-03" {
			day3 += e.Miss + e.CacheRead + e.CacheCreate + e.Output
		}
	}
	if day3 != 9 {
		t.Fatalf("overlap day=%d", day3)
	}
	again, err := lg.SupplementNarrowWindow(got)
	if err != nil {
		t.Fatal(err)
	}
	if tokensOf(again) != 163 {
		t.Fatalf("second supplement total=%d", tokensOf(again))
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || tokensOf(chained) != 163 {
		t.Fatalf("chain ok=%v err=%v total=%d", ok, err, tokensOf(chained))
	}
}

func TestVerifiedEarlyDaySurvivesAShorterLaterWindow(t *testing.T) {
	lg := openTest(t)
	early := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-21",
		Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
		Miss:      50, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	inside := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-27",
		Timestamp: time.Date(2025, 10, 27, 12, 0, 0, 0, time.UTC),
		Miss:      9, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), []event.UsageEvent{early, inside}); err != nil {
		t.Fatal(err)
	}
	replaced := event.UsageEvent{
		Source: "cursor", RequestID: "new-2025-10-27",
		Timestamp: time.Date(2025, 10, 27, 12, 0, 0, 0, time.UTC),
		Miss:      4, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	added := event.UsageEvent{
		Source: "cursor", RequestID: "new-2025-10-26",
		Timestamp: time.Date(2025, 10, 26, 12, 0, 0, 0, time.UTC),
		Miss:      3, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.NoteCoveredWindow("cursor", "2025-10-26", "2025-10-31", []event.UsageEvent{replaced, added}); err != nil {
		t.Fatal(err)
	}
	later := event.UsageEvent{
		Source: "cursor", RequestID: "new-2026-11-01",
		Timestamp: time.Date(2026, 11, 1, 12, 0, 0, 0, time.UTC),
		Miss:      8, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-11-01", "2026-11-30", []event.UsageEvent{later}); err != nil {
		t.Fatal(err)
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	got := map[string]int64{}
	for _, e := range chained {
		day := e.Timestamp.UTC().Format("2006-01-02")
		got[day] += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	if got["2025-10-21"] != 50 || got["2025-10-27"] != 4 || got["2025-10-26"] != 3 || got["2026-11-01"] != 8 {
		t.Fatalf("days=%v", got)
	}
	for _, day := range []string{"2025-10-22", "2025-10-23", "2025-10-24", "2025-10-25"} {
		if _, ok := got[day]; ok {
			t.Fatalf("missing day %s was fabricated", day)
		}
	}
	if tokensOf(chained) != 65 {
		t.Fatalf("total=%d", tokensOf(chained))
	}
}

func TestCoveredRangesIgnoreARetainedSnapshot(t *testing.T) {
	lg := openTest(t)
	early := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-21",
		Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
		Miss:      50, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), []event.UsageEvent{early}); err != nil {
		t.Fatal(err)
	}
	month := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-27",
		Timestamp: time.Date(2025, 10, 27, 12, 0, 0, 0, time.UTC),
		Miss:      4, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.NoteCoveredWindow("cursor", "2025-10-26", "2025-10-31", []event.UsageEvent{month}); err != nil {
		t.Fatal(err)
	}
	ranges, err := lg.CoveredRanges("cursor")
	if err != nil {
		t.Fatal(err)
	}
	if len(ranges) != 1 || ranges[0].From != "2025-10-26" || ranges[0].To != "2025-10-31" {
		t.Fatalf("ranges=%+v", ranges)
	}
	extra, err := lg.AccountSupplement("cursor", DayRange{From: "2025-10-26", To: "2026-11-01"}, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]event.UsageEvent{}
	for _, e := range extra {
		got[e.Timestamp.UTC().Format("2006-01-02")] = e
	}
	if e, ok := got["2025-10-27"]; !ok || e.Derivation != event.DeriveProviderAPI || e.Quality != event.QualityAuthoritative || e.Miss != 4 {
		t.Fatalf("cached month day=%+v", got["2025-10-27"])
	}
	if e, ok := got["2025-10-21"]; !ok || e.Derivation != event.DeriveRetained || e.Quality != event.QualityDegraded || e.Miss != 50 {
		t.Fatalf("snapshot day=%+v", got["2025-10-21"])
	}
	for _, day := range []string{"2025-10-22", "2025-10-23", "2025-10-24", "2025-10-25"} {
		if _, ok := got[day]; ok {
			t.Fatalf("missing day %s was fabricated", day)
		}
	}
}

func TestOldDailyRowsReplaceByWindowAndKeepTheUnknownGap(t *testing.T) {
	lg := openTest(t)
	oct21 := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-21",
		Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
		Miss:      539187, CacheRead: 5675130, CacheCreate: 77078, Output: 55585,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if oct21.Miss+oct21.CacheRead+oct21.CacheCreate+oct21.Output != 6346980 {
		t.Fatalf("oct 21 fixture=%d", oct21.Miss+oct21.CacheRead+oct21.CacheCreate+oct21.Output)
	}
	old := []event.UsageEvent{oct21}
	day := time.Date(2025, 10, 26, 12, 0, 0, 0, time.UTC)
	for len(old) < 166 {
		old = append(old, event.UsageEvent{
			Source: "cursor", RequestID: "day-" + day.Format("2006-01-02"),
			Timestamp: day, Miss: 1,
			Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		})
		day = day.AddDate(0, 0, 1)
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), old); err != nil {
		t.Fatal(err)
	}
	replaced := []event.UsageEvent{
		{
			Source: "cursor", RequestID: "new-2025-10-26",
			Timestamp: time.Date(2025, 10, 26, 12, 0, 0, 0, time.UTC),
			Miss:      3, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
		{
			Source: "cursor", RequestID: "new-2025-10-27",
			Timestamp: time.Date(2025, 10, 27, 12, 0, 0, 0, time.UTC),
			Miss:      4, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
	}
	if err := lg.NoteCoveredWindow("cursor", "2025-10-26", "2025-10-31", replaced); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteWindowAudit("cursor", WindowAudit{
		From: "2025-10-26", To: "2025-10-31",
		Declared: 2, RawRows: 2, Deduped: 2, Days: 2, Miss: 7, Validation: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	assertChain := func() {
		t.Helper()
		chained, ok, err := lg.ChainedRetainedEvents("cursor")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		got := map[string]int64{}
		for _, e := range chained {
			got[e.Timestamp.UTC().Format("2006-01-02")] += e.Miss + e.CacheRead + e.CacheCreate + e.Output
		}
		if got["2025-10-21"] != 6346980 {
			t.Fatalf("oct 21=%d", got["2025-10-21"])
		}
		for _, day := range []string{"2025-10-22", "2025-10-23", "2025-10-24", "2025-10-25"} {
			if _, ok := got[day]; ok {
				t.Fatalf("unknown day %s was fabricated", day)
			}
		}
		if got["2025-10-26"] != 3 || got["2025-10-27"] != 4 {
			t.Fatalf("replaced days=%v", got)
		}
		if got["2025-10-28"] != 0 {
			t.Fatalf("day inside the new window survived the replacement: %d", got["2025-10-28"])
		}
	}
	assertChain()
	var rows int
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&rows); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2025-10-26", "2025-10-31", replaced); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteWindowAudit("cursor", WindowAudit{
		From: "2025-10-26", To: "2025-10-31",
		Declared: 2, RawRows: 2, Deduped: 2, Days: 2, Miss: 7, Validation: "ok",
	}); err != nil {
		t.Fatal(err)
	}
	var rowsAgain, audits int
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&rowsAgain); err != nil {
		t.Fatal(err)
	}
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_window_audits`).Scan(&audits); err != nil {
		t.Fatal(err)
	}
	if rowsAgain != rows || audits != 1 {
		t.Fatalf("rows %d→%d audits=%d", rows, rowsAgain, audits)
	}
	assertChain()
	stored, err := lg.WindowAudit("cursor", "2025-10-26", "2025-10-31")
	if err != nil || stored.Validation != "ok" || stored.Declared != 2 || stored.RawRows != 2 || stored.Deduped != 2 || stored.Miss != 7 {
		t.Fatalf("audit=%+v err=%v", stored, err)
	}
	missing, err := lg.WindowAudit("cursor", "2025-11-01", "2025-11-30")
	if err != nil || missing.Validation != "missing" || missing.Declared != 0 || missing.Miss != 0 {
		t.Fatalf("missing audit=%+v err=%v", missing, err)
	}
	if err := lg.NoteWindowAudit("cursor", WindowAudit{From: "2025-11-01", To: "2025-11-30", Validation: "missing"}); err == nil {
		t.Fatal("a missing audit was stored")
	}
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_window_audits`).Scan(&audits); err != nil || audits != 1 {
		t.Fatalf("audits=%d err=%v", audits, err)
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
