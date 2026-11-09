package ledger

import (
	"math"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestUnallocatedHistorySharesCanonicalUnknownVendor(t *testing.T) {
	lg, err := Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	stamp := time.Date(2026, 1, 1, 12, 0, 0, 0, time.UTC)
	base := event.UsageEvent{Source: "cursor", Timestamp: stamp, Miss: 20}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", stamp, []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	measured := base
	measured.Miss = 10
	measured.Model = "default"
	measured.Vendor = "unknown"
	measured.Derivation = event.DeriveProviderAPI
	if _, err := lg.BackfillAttribution("cursor", []event.UsageEvent{measured}); err != nil {
		t.Fatal(err)
	}
	rows, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil {
		t.Fatal(err)
	}
	by := map[string]int64{}
	for _, e := range rows {
		by[e.Vendor] += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	if len(by) != 1 || by["unknown"] != 20 {
		t.Fatalf("unknown vendor split across ids: %v", by)
	}
}

// These checks catch restoring a day total without its measured attribution,
// and assigning every measured request to the synthetic noon UTC timestamp.
func TestObservationAttributionSurvivesEveryRestore(t *testing.T) {
	for _, writer := range []string{"api", "window", "snapshot"} {
		t.Run(writer, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "ledger.db")
			lg, err := Open(path)
			if err != nil {
				t.Fatal(err)
			}
			one := apiEvent(1, 11)
			one.Model, one.Vendor, one.RequestID = "model-a", "vendor-a", "secret/raw/request"
			one.Provenance = "cursor_filtered_usage"
			one.Timestamp = time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)
			two := one
			two.Model, two.Vendor, two.RequestID, two.Miss, two.Output = "model-b", "vendor-b", "other-request", 3, 7
			two.Provenance = "cursor_billing_v2"
			evs := []event.UsageEvent{one, two}
			switch writer {
			case "api":
				err = lg.NoteAPI(evs)
			case "window":
				err = lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-02", evs)
			case "snapshot":
				err = lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), evs)
			}
			if err != nil {
				t.Fatal(err)
			}
			if err := lg.Close(); err != nil {
				t.Fatal(err)
			}
			lg, err = OpenReadOnly(path)
			if err != nil {
				t.Fatal(err)
			}
			defer lg.Close()
			latest, _, err := lg.LatestRetainedEvents("cursor")
			if err != nil {
				t.Fatal(err)
			}
			chain, _, err := lg.ChainedRetainedEvents("cursor")
			if err != nil {
				t.Fatal(err)
			}
			outside, err := lg.DaysOutside("cursor", nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			account, err := lg.AccountSupplement("cursor", DayRange{From: "2026-01-01", To: "2026-01-02"}, nil, nil)
			if err != nil {
				t.Fatal(err)
			}
			narrow, err := lg.SupplementNarrowWindow([]event.UsageEvent{apiEvent(3, 1)})
			if err != nil {
				t.Fatal(err)
			}
			for name, got := range map[string][]event.UsageEvent{"latest": latest, "chain": chain, "outside": outside, "account": account, "narrow": narrow} {
				var a, b int
				for _, e := range got {
					if e.Model == "model-a" && e.Vendor == "vendor-a" && e.Miss == 11 && e.Timestamp.Equal(one.Timestamp) && e.Provenance == "cursor_filtered_usage" {
						a++
					}
					if e.Model == "model-b" && e.Vendor == "vendor-b" && e.Miss == 3 && e.Output == 7 && e.Timestamp.Equal(two.Timestamp) && e.Provenance == "cursor_billing_v2" {
						b++
					}
					if strings.Contains(e.RequestID, "secret/raw/request") {
						t.Fatalf("%s leaked raw request id", name)
					}
				}
				if a != 1 || b != 1 {
					t.Errorf("%s lost attribution: %+v", name, got)
				}
			}
		})
	}
}

func TestBackfillPreservesEndpointProvenanceAndRemainsRepeatable(t *testing.T) {
	for _, provenance := range []string{"cursor_dashboard_csv", "cursor_billing_v2", "cursor_filtered_usage", "/private/untrusted/path"} {
		t.Run(provenance, func(t *testing.T) {
			lg := openTest(t)
			base := apiEvent(1, 100)
			base.Model = ""
			if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
				t.Fatal(err)
			}
			detail := base
			detail.Model, detail.Miss, detail.Provenance = "measured-model", 40, provenance
			for i := 0; i < 2; i++ {
				result, err := lg.BackfillAttribution("cursor", []event.UsageEvent{detail})
				if err != nil || result.AllocatedDays != 1 || result.RejectedDays != 0 {
					t.Fatalf("repeat %d result=%+v err=%v", i, result, err)
				}
				got, _, err := lg.ChainedRetainedEvents("cursor")
				if err != nil || len(got) != 2 || tokensOf(got) != 100 {
					t.Fatalf("got=%+v err=%v", got, err)
				}
				want := provenance
				if provenance == "/private/untrusted/path" {
					want = "attribution_backfill"
				}
				for _, e := range got {
					if e.Model == "measured-model" && e.Provenance != want {
						t.Fatalf("endpoint evidence lost or untrusted value retained: %+v", e)
					}
					if e.Model == "" && e.Provenance != "history_unallocated" {
						t.Fatalf("unallocated residual evidence %+v", e)
					}
				}
			}
		})
	}
}

func TestObservationProvenanceUsesFixedAllowlist(t *testing.T) {
	for _, provenance := range []string{"cursor_dashboard_csv", "cursor_billing_v2", "cursor_filtered_usage", "/private/untrusted/path", "eyJcredential", "attribution_backfill"} {
		t.Run(provenance, func(t *testing.T) {
			lg := openTest(t)
			e := apiEvent(1, 10)
			e.Provenance = provenance
			if err := lg.NoteAPI([]event.UsageEvent{e}); err != nil {
				t.Fatal(err)
			}
			want := provenance
			if provenance == "/private/untrusted/path" || provenance == "eyJcredential" || provenance == "attribution_backfill" {
				want = "api"
			}
			got, _, err := lg.LatestRetainedEvents("cursor")
			if err != nil || len(got) != 1 || got[0].Provenance != want {
				t.Fatalf("provenance restored=%+v want=%q err=%v", got, want, err)
			}
			var stored string
			if err := lg.db.QueryRow(`SELECT provenance FROM api_observation_details`).Scan(&stored); err != nil || stored != want {
				t.Fatalf("stored provenance=%q want=%q err=%v", stored, want, err)
			}
		})
	}
}

func TestRestoredProvenanceCannotExposeUntrustedStoredValues(t *testing.T) {
	lg := openTest(t)
	e := apiEvent(1, 10)
	if err := lg.NoteAPI([]event.UsageEvent{e}); err != nil {
		t.Fatal(err)
	}
	if _, err := lg.db.Exec(`UPDATE api_observation_details SET provenance='/private/untrusted/path'`); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Provenance != "" || strings.Contains(got[0].RequestID, "/private/untrusted/path") {
		t.Fatalf("unsafe persisted provenance exposed %+v err=%v", got, err)
	}
}

func TestSameComponentsWithNewModelReplaceAttribution(t *testing.T) {
	for _, writer := range []string{"api", "window", "snapshot"} {
		t.Run(writer, func(t *testing.T) {
			lg := openTest(t)
			e := apiEvent(1, 9)
			write := func() error {
				switch writer {
				case "api":
					return lg.NoteAPI([]event.UsageEvent{e})
				case "window":
					return lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-02", []event.UsageEvent{e})
				default:
					return lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{e})
				}
			}
			if err := write(); err != nil {
				t.Fatal(err)
			}
			e.Model, e.Vendor = "new-model", "new-vendor"
			if err := write(); err != nil {
				t.Fatal(err)
			}
			if err := write(); err != nil {
				t.Fatal(err)
			}
			got, _, err := lg.ChainedRetainedEvents("cursor")
			if err != nil || len(got) != 1 || got[0].Model != "new-model" || got[0].Vendor != "new-vendor" || tokensOf(got) != 9 {
				t.Fatalf("got=%+v err=%v", got, err)
			}
			var n int
			if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&n); err != nil || n != 2 {
				t.Fatalf("observations=%d err=%v", n, err)
			}
		})
	}
}

func TestReconfirmedAPIAttributionCanReturnToEarlierPayload(t *testing.T) {
	for _, writer := range []string{"api", "window", "observation"} {
		t.Run(writer, func(t *testing.T) {
			lg := openTest(t)
			e := apiEvent(1, 10)
			when := time.Date(2026, 1, 1, 0, 0, 0, 115920000, time.UTC)
			write := func() error {
				switch writer {
				case "api":
					return lg.NoteAPI([]event.UsageEvent{e})
				case "window":
					return lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-01", []event.UsageEvent{e})
				default:
					return lg.SaveObservation("cursor", "api", "complete", when, []event.UsageEvent{e})
				}
			}
			for _, model := range []string{"model-a", "model-b", "model-a"} {
				e.Model = model
				if err := write(); err != nil {
					t.Fatal(err)
				}
				when = when.Add(time.Microsecond)
			}
			for _, read := range []func(string) ([]event.UsageEvent, bool, error){lg.LatestRetainedEvents, lg.ChainedRetainedEvents} {
				got, ok, err := read("cursor")
				if err != nil || !ok || len(got) != 1 || got[0].Model != "model-a" || tokensOf(got) != 10 {
					t.Fatalf("reconfirmation lost %+v ok=%v err=%v", got, ok, err)
				}
			}
			var count int
			if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&count); err != nil || count != 2 {
				t.Fatalf("reconfirmation duplicated payload rows=%d err=%v", count, err)
			}
			var before, after string
			if err := lg.db.QueryRow(`SELECT observed_at FROM api_observations WHERE payload_hash=(SELECT payload_hash FROM api_observation_details WHERE model='model-a' LIMIT 1)`).Scan(&before); err != nil {
				t.Fatal(err)
			}
			if err := write(); err != nil {
				t.Fatal(err)
			}
			if err := lg.db.QueryRow(`SELECT observed_at FROM api_observations WHERE payload_hash=(SELECT payload_hash FROM api_observation_details WHERE model='model-a' LIMIT 1)`).Scan(&after); err != nil || before != after {
				t.Fatalf("duplicate current response changed ordering %q -> %q err=%v", before, after, err)
			}
		})
	}
}

func TestReconfirmationDoesNotPromoteOldImportsOrUnrelatedWindows(t *testing.T) {
	lg := openTest(t)
	a := apiEvent(1, 10)
	a.Model = "model-a"
	b := a
	b.Model = "model-b"
	when := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	if err := lg.SaveObservation("cursor", "api", "complete", when, []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	if err := lg.SaveObservation("cursor", "api", "complete", when.Add(2*time.Hour), []event.UsageEvent{b}); err != nil {
		t.Fatal(err)
	}
	if err := lg.SaveObservation("cursor", "api", "complete", when.Add(time.Hour), []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "model-b" {
		t.Fatalf("older import promoted %+v err=%v", got, err)
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", when.Add(-time.Hour), []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", when.Add(24*time.Hour), []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	got, _, err = lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "model-b" {
		t.Fatalf("snapshot reimport promoted %+v err=%v", got, err)
	}

	jan, feb := apiEvent(2, 20), apiEvent(3, 30)
	jan.Model, feb.Model = "jan-model", "feb-model"
	feb.Timestamp = time.Date(2026, 2, 3, 3, 0, 0, 0, time.UTC)
	if err := lg.NoteCoveredWindow("other", "2026-01-01", "2026-01-31", []event.UsageEvent{jan}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("other", "2026-02-01", "2026-02-28", []event.UsageEvent{feb}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("other", "2026-01-01", "2026-01-31", []event.UsageEvent{jan}); err != nil {
		t.Fatal(err)
	}
	got, _, err = lg.LatestRetainedEvents("other")
	if err != nil || len(got) != 1 || got[0].Model != "feb-model" {
		t.Fatalf("unrelated window changed ordering %+v err=%v", got, err)
	}
}

func TestReconfirmedCoveredWindowReplacesOnlyItsOwnRange(t *testing.T) {
	lg := openTest(t)
	a := apiEvent(1, 10)
	a.Model = "model-a"
	b := apiEvent(2, 20)
	b.Model = "model-b"
	c := apiEvent(3, 30)
	c.Model = "model-c"
	if err := lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-02", []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-01-02", "2026-01-02", []event.UsageEvent{b}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-01-03", "2026-01-03", []event.UsageEvent{c}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-02", []event.UsageEvent{a}); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 2 || tokensOf(got) != 40 || got[0].Model != "model-a" || got[1].Model != "model-c" {
		t.Fatalf("range reconfirmation failed %+v err=%v", got, err)
	}
}

func TestBackfillAttributionConservesEveryComponentAndResidual(t *testing.T) {
	lg := openTest(t)
	base := apiEvent(1, 100)
	base.Model, base.CacheRead, base.CacheCreate, base.Output = "", 80, 30, 20
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	real := base
	real.Model, real.Vendor, real.RequestID = "real-model", "real-vendor", "raw/secret/id"
	real.Miss, real.CacheRead, real.CacheCreate, real.Output = 60, 80, 10, 5
	real.Timestamp = time.Date(2026, 1, 1, 23, 30, 0, 0, time.UTC)
	for i := 0; i < 2; i++ {
		result, err := lg.BackfillAttribution("cursor", []event.UsageEvent{real})
		if err != nil || result.AllocatedDays != 1 || result.RejectedDays != 0 || result.MissingDays != 0 {
			t.Fatalf("result=%+v err=%v", result, err)
		}
		got, _, err := lg.ChainedRetainedEvents("cursor")
		if err != nil || len(got) != 2 || tokensOf(got) != 230 {
			t.Fatalf("got=%+v err=%v", got, err)
		}
		var assigned, residual *event.UsageEvent
		for j := range got {
			if got[j].Model == "real-model" {
				assigned = &got[j]
			} else if got[j].Model == "" {
				residual = &got[j]
			}
		}
		if assigned == nil || assigned.Vendor != "real-vendor" || !assigned.Timestamp.Equal(real.Timestamp) || assigned.Miss != 60 || assigned.CacheRead != 80 || assigned.CacheCreate != 10 || assigned.Output != 5 {
			t.Fatalf("assigned=%+v", assigned)
		}
		if residual == nil || residual.Miss != 40 || residual.CacheRead != 0 || residual.CacheCreate != 20 || residual.Output != 15 || !strings.Contains(residual.RequestID, "history-unallocated") {
			t.Fatalf("residual=%+v", residual)
		}
	}
	var observations int
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&observations); err != nil || observations != 1 {
		t.Fatalf("backfill rewrote observations=%d err=%v", observations, err)
	}
	_, _, permit, err := lg.CorrectionPermit()
	if err != nil || permit {
		t.Fatalf("attribution granted correction permit=%v err=%v", permit, err)
	}
}

func TestBackfillRejectsComponentOverflowAndMissingDays(t *testing.T) {
	lg := openTest(t)
	base := apiEvent(1, 100)
	base.Model, base.CacheRead = "", 5
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	bad := base
	bad.Model, bad.Miss, bad.CacheRead = "new-model", 1, 6 // total fits, cache-read does not.
	missing := apiEvent(2, 7)
	missing.RequestID = "missing-day-request"
	result, err := lg.BackfillAttribution("cursor", []event.UsageEvent{bad, missing})
	if err != nil || result.AllocatedDays != 0 || result.RejectedDays != 1 || result.MissingDays != 1 {
		t.Fatalf("result=%+v err=%v", result, err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "" || got[0].Miss != 100 || got[0].CacheRead != 5 {
		t.Fatalf("changed rejected day: %+v err=%v", got, err)
	}
}

func TestBackfillCannotSurviveChangedBaselineOrReplaceNewAPIDetail(t *testing.T) {
	lg := openTest(t)
	base := apiEvent(1, 10)
	base.Model = ""
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	detail := base
	detail.Model = "backfilled-model"
	if _, err := lg.BackfillAttribution("cursor", []event.UsageEvent{detail}); err != nil {
		t.Fatal(err)
	}
	if _, err := lg.db.Exec(`UPDATE api_observation_days SET miss=11 WHERE source='cursor'`); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "" || tokensOf(got) != 11 {
		t.Fatalf("stale checksum applied %+v err=%v", got, err)
	}
	fresh := detail
	fresh.Model, fresh.Miss = "new-response-model", 10
	if err := lg.NoteCoveredWindow("cursor", "2026-01-01", "2026-01-01", []event.UsageEvent{fresh}); err != nil {
		t.Fatal(err)
	}
	if _, err := lg.BackfillAttribution("cursor", []event.UsageEvent{detail}); err != nil {
		t.Fatal(err)
	}
	got, _, err = lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "new-response-model" || tokensOf(got) != 10 {
		t.Fatalf("new response overridden %+v err=%v", got, err)
	}
}

func TestObservationOrderingHandlesTrimmedFractionalSeconds(t *testing.T) {
	lg := openTest(t)
	e := apiEvent(1, 10)
	when := time.Date(2026, 1, 1, 0, 0, 0, 115920000, time.UTC)
	if err := lg.SaveObservation("cursor", "api", "complete", when, []event.UsageEvent{e}); err != nil {
		t.Fatal(err)
	}
	e.Model = "new-model"
	if err := lg.SaveObservation("cursor", "api", "complete", when.Add(time.Microsecond), []event.UsageEvent{e}); err != nil {
		t.Fatal(err)
	}
	for _, read := range []func(string) ([]event.UsageEvent, bool, error){lg.LatestRetainedEvents, lg.ChainedRetainedEvents} {
		got, _, err := read("cursor")
		if err != nil || len(got) != 1 || got[0].Model != "new-model" {
			t.Fatalf("newest observation not selected %+v err=%v", got, err)
		}
	}
}

func TestBackfillRejectsOverflowWithoutWritingAttribution(t *testing.T) {
	lg := openTest(t)
	base := apiEvent(1, 10)
	base.Model = ""
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	large := base
	large.Model, large.Miss, large.RequestID = "new-model", math.MaxInt64, ""
	_, err := lg.BackfillAttribution("cursor", []event.UsageEvent{large, large})
	if err == nil {
		t.Fatal("overflowing day components accepted")
	}
	got, _, readErr := lg.ChainedRetainedEvents("cursor")
	if readErr != nil || len(got) != 1 || got[0].Model != "" || tokensOf(got) != 10 {
		t.Fatalf("invalid backfill changed baseline %+v err=%v", got, readErr)
	}
}

func TestBackfillDeduplicatesOnlyStableRequestIDs(t *testing.T) {
	for _, requestID := range []string{"stable-request", ""} {
		t.Run(requestID, func(t *testing.T) {
			lg := openTest(t)
			base := apiEvent(1, 100)
			base.Model = ""
			if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
				t.Fatal(err)
			}
			detail := base
			detail.Model, detail.RequestID, detail.Miss = "measured-model", requestID, 40
			result, err := lg.BackfillAttribution("cursor", []event.UsageEvent{detail, detail})
			if err != nil || result.AllocatedDays != 1 {
				t.Fatalf("result=%+v err=%v", result, err)
			}
			got, _, err := lg.ChainedRetainedEvents("cursor")
			if err != nil || tokensOf(got) != 100 {
				t.Fatalf("total changed %+v err=%v", got, err)
			}
			var measured, unallocated int64
			requestIDs := map[string]bool{}
			for _, e := range got {
				if e.Model != "" {
					measured += e.Miss
				} else {
					unallocated += e.Miss
				}
				if requestIDs[e.RequestID] {
					t.Fatalf("independent requests share restored identity: %+v", got)
				}
				requestIDs[e.RequestID] = true
			}
			wantMeasured, wantUnallocated := int64(40), int64(60)
			if requestID == "" {
				wantMeasured, wantUnallocated = 80, 20
			}
			if measured != wantMeasured || unallocated != wantUnallocated {
				t.Fatalf("measured=%d unallocated=%d, want %d/%d", measured, unallocated, wantMeasured, wantUnallocated)
			}
		})
	}
}

func TestBackfillRejectsConflictingStableRequestWithoutAnyWrites(t *testing.T) {
	changes := map[string]func(*event.UsageEvent){
		"miss":         func(e *event.UsageEvent) { e.Miss++ },
		"cache-read":   func(e *event.UsageEvent) { e.CacheRead++ },
		"cache-create": func(e *event.UsageEvent) { e.CacheCreate++ },
		"output":       func(e *event.UsageEvent) { e.Output++ },
		"model":        func(e *event.UsageEvent) { e.Model = "different-model" },
		"vendor":       func(e *event.UsageEvent) { e.Vendor = "different-vendor" },
		"timestamp":    func(e *event.UsageEvent) { e.Timestamp = e.Timestamp.Add(time.Second) },
		"day":          func(e *event.UsageEvent) { e.Timestamp = e.Timestamp.AddDate(0, 0, 1) },
	}
	for name, change := range changes {
		t.Run(name, func(t *testing.T) {
			lg := openTest(t)
			base := apiEvent(1, 100)
			base.Model, base.CacheRead, base.CacheCreate, base.Output = "", 20, 20, 20
			if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
				t.Fatal(err)
			}
			detail := base
			detail.Model, detail.Vendor, detail.Miss, detail.CacheRead, detail.CacheCreate, detail.Output = "model", "vendor", 10, 1, 1, 1
			conflict := detail
			change(&conflict)
			if _, err := lg.BackfillAttribution("cursor", []event.UsageEvent{detail, conflict}); err == nil {
				t.Fatal("conflicting request payload accepted")
			}
			got, _, err := lg.ChainedRetainedEvents("cursor")
			if err != nil || len(got) != 1 || got[0].Model != "" || tokensOf(got) != 160 {
				t.Fatalf("conflict changed baseline %+v err=%v", got, err)
			}
		})
	}
}

func TestDayOnlyLedgerRemainsReadableWithoutMigration(t *testing.T) {
	path := filepath.Join(t.TempDir(), "old.db")
	lg, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	base := apiEvent(1, 10)
	base.Model = ""
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{base}); err != nil {
		t.Fatal(err)
	}
	// Recreate the older schema, which has daily totals but no attribution tables.
	for _, table := range []string{"api_observation_details", "api_attribution_days", "api_attribution_details"} {
		if _, err := lg.db.Exec(`DROP TABLE ` + table); err != nil {
			t.Fatal(err)
		}
	}
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	lg, err = OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	got, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || len(got) != 1 || got[0].Model != "" || tokensOf(got) != 10 || !strings.Contains(got[0].RequestID, "history-unallocated") {
		t.Fatalf("old ledger unavailable or mislabeled %+v ok=%v err=%v", got, ok, err)
	}
}

func TestBackfillPersistsAcrossReopenAndMonthReplacement(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	lg, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	jan := apiEvent(31, 10)
	jan.Model = ""
	feb := jan
	feb.RequestID = "feb-request"
	feb.Timestamp = time.Date(2026, 2, 1, 23, 30, 0, 0, time.UTC)
	feb.Miss = 20
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Now(), []event.UsageEvent{jan, feb}); err != nil {
		t.Fatal(err)
	}
	jan.Model, jan.Vendor = "jan-model", "jan-vendor"
	feb.Model, feb.Vendor = "feb-model", "feb-vendor"
	if _, err := lg.BackfillAttribution("cursor", []event.UsageEvent{jan, feb}); err != nil {
		t.Fatal(err)
	}
	if err := lg.Close(); err != nil {
		t.Fatal(err)
	}
	lg, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	feb.Model, feb.Miss = "fresh-feb-model", 7
	if err := lg.NoteCoveredWindow("cursor", "2026-02-01", "2026-02-28", []event.UsageEvent{feb}); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 2 || tokensOf(got) != 17 {
		t.Fatalf("cross-month attribution duplicated or lost %+v err=%v", got, err)
	}
	if got[0].Model != "jan-model" || got[0].Vendor != "jan-vendor" || got[1].Model != "fresh-feb-model" {
		t.Fatalf("model replacement %+v", got)
	}
}

func TestSupersededAttributionDoesNotBlockCurrentChain(t *testing.T) {
	lg := openTest(t)
	old := apiEvent(1, 10)
	if err := lg.SaveObservation("cursor", "api", "complete", time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC), []event.UsageEvent{old}); err != nil {
		t.Fatal(err)
	}
	// The obsolete detail is unreadable, but the current day is supplied in
	// full by a later response and does not depend on that old model row.
	if _, err := lg.db.Exec(`UPDATE api_observation_details SET ts='invalid' WHERE source='cursor'`); err != nil {
		t.Fatal(err)
	}
	current := old
	current.Model, current.Miss = "current-model", 20
	if err := lg.SaveObservation("cursor", "api", "complete", time.Date(2026, 1, 2, 0, 0, 0, 0, time.UTC), []event.UsageEvent{current}); err != nil {
		t.Fatal(err)
	}
	got, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(got) != 1 || got[0].Model != "current-model" || tokensOf(got) != 20 {
		t.Fatalf("superseded detail blocked current chain %+v err=%v", got, err)
	}
}

func TestAttributionHydrationSpansMultipleSQLBatches(t *testing.T) {
	lg := openTest(t)
	start := time.Date(2025, 1, 1, 23, 30, 0, 0, time.UTC)
	rows := make([]event.UsageEvent, 0, 602)
	for day := 0; day < 602; day++ {
		e := apiEvent(1, 1)
		e.Timestamp = start.AddDate(0, 0, day)
		e.Model, e.Vendor, e.RequestID = "model-"+strconv.Itoa(day), "vendor", strconv.Itoa(day)
		rows = append(rows, e)
	}
	if err := lg.NoteAPI(rows); err != nil {
		t.Fatal(err)
	}
	got, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || len(got) != 602 || tokensOf(got) != 602 {
		t.Fatalf("batch lost rows=%d ok=%v err=%v", len(got), ok, err)
	}
	for day, e := range got {
		if e.Model != "model-"+strconv.Itoa(day) || e.Vendor != "vendor" || !e.Timestamp.Equal(start.AddDate(0, 0, day)) {
			t.Fatalf("batch attribution mismatch at day %d: %+v", day, e)
		}
	}
}

func BenchmarkChainedAttributionOverlappingObservations(b *testing.B) {
	lg, err := Open(filepath.Join(b.TempDir(), "ledger.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer lg.Close()
	start := time.Date(2026, 1, 1, 1, 0, 0, 0, time.UTC)
	for version := 0; version < 91; version++ {
		rows := make([]event.UsageEvent, 0, 182*3)
		for day := 0; day < 182; day++ {
			for request := 0; request < 3; request++ {
				rows = append(rows, event.UsageEvent{Source: "cursor", Model: "model-" + strconv.Itoa(version), Vendor: "vendor",
					RequestID: strconv.Itoa(day) + "-" + strconv.Itoa(request), Timestamp: start.AddDate(0, 0, day).Add(time.Duration(request) * time.Hour),
					Miss: 1, Derivation: event.DeriveProviderAPI, Quality: event.QualityAuthoritative, SkipRequest: true})
			}
		}
		if err := lg.SaveObservation("cursor", "api", "complete", start.Add(time.Duration(version)*time.Minute), rows); err != nil {
			b.Fatal(err)
		}
	}
	b.ReportAllocs()
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		rows, ok, err := lg.ChainedRetainedEvents("cursor")
		if err != nil || !ok || len(rows) != 546 {
			b.Fatalf("rows=%d ok=%v err=%v", len(rows), ok, err)
		}
	}
}
