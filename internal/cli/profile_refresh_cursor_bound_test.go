package cli

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

func TestRefreshRetainsCursorOnlyForExplicitlyConfirmedAccount(t *testing.T) {
	home := testhome.New(t.TempDir())
	path, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	day := time.Date(2026, 11, 7, 12, 0, 0, 0, time.UTC)
	stored := []event.UsageEvent{{Source: "cursor", RequestID: "api-day", Timestamp: day,
		Miss: 1_100_000, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true}}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", day.Add(10*time.Minute), stored); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	app := &App{}
	local := func() []event.UsageEvent {
		return []event.UsageEvent{{Source: "cursor", RequestID: "session-local", SessionID: "sess",
			Timestamp: day, Miss: 88, Output: 12, Quality: event.QualityDegraded, Derivation: event.DeriveRaw}}
	}
	check := func(want bool, errs []string) {
		t.Helper()
		evs, restored := app.retainAccountObservations(home, local(), errs, nil)
		if restored != want {
			t.Fatalf("confirmed=%t restored=%t events=%+v", want, restored, evs)
		}
		if !want {
			if len(evs) != 1 || evs[0].Miss != 88 {
				t.Fatalf("unbound history must stay separate: %+v", evs)
			}
			return
		}
		if len(evs) != 2 || evs[0].Miss != 0 || evs[0].Output != 0 ||
			evs[1].Miss != 1_100_000 || evs[1].Derivation != event.DeriveRetained || !evs[1].SkipRequest {
			t.Fatalf("confirmed history missing/double-counted: %+v", evs)
		}
	}
	check(false, nil)
	check(false, []string{"cursor: ENDPOINT_UNAVAILABLE"})
	lg, _ = ledger.Open(path)
	if err := lg.SetMeta(cursor.AccountSubjectMeta, "auth0|user_1"); err != nil {
		t.Fatal(err)
	}
	if err := lg.SetMeta(cursor.HistoryDecisionMeta, cursor.HistoryConfirmed); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	check(false, nil) // an explicit decision without its subject is not consent
	lg, _ = ledger.Open(path)
	if err := lg.SetMeta(cursor.HistoryDecisionSubjectMeta, "auth0|other_account"); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	check(false, []string{"cursor: API unavailable"})
	lg, _ = ledger.Open(path)
	if err := lg.SetMeta(cursor.HistoryDecisionSubjectMeta, "auth0|user_1"); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	check(true, nil) // silent provider gap must preserve confirmed measured history
	check(true, []string{"cursor: ENDPOINT_UNAVAILABLE"})
	fresh := event.UsageEvent{Source: "cursor", RequestID: "live", Timestamp: day,
		Miss: 1234, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true}
	got, restored := app.retainAccountObservations(home, append(local(), fresh), nil, nil)
	if restored || len(got) != 2 || got[1].Miss != 1234 {
		t.Fatalf("fresh API must not be replaced by stale history: restored=%v events=%+v", restored, got)
	}
}
