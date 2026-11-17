package cursor

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestFilteredInvalidCountIsNotAConfirmedZero(t *testing.T) {
	for _, count := range []string{`null`, `"redacted-count-value"`, `false`, `0.5`, `-1`} {
		t.Run(count, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				fmt.Fprintf(w, `{"totalUsageEventsCount":%s,"usageEventsDisplay":[]}`, count)
			}))
			defer srv.Close()
			got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchFiltered(fakeJWT, 1, 2)
			if !errors.Is(err, errSchemaChanged) || len(got.events) != 0 {
				t.Fatalf("invalid count was accepted: got=%+v err=%v", got, err)
			}
		})
	}
}

func TestRefreshRejectsDifferentAccountBeforeRequest(t *testing.T) {
	refresh := testJWT(t, `{"sub":"test-user","type":"refresh_token"}`)
	replacement := testJWT(t, `{"sub":"other-user","type":"session"}`)
	usageRequests := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == oauthPath {
			_ = json.NewEncoder(w).Encode(map[string]string{"access_token": replacement})
			return
		}
		usageRequests++
		if r.Header.Get("Authorization") == "Bearer "+fakeJWT {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{"totalUsageEventsCount":0,"usageEventsDisplay":[]}`)
	}))
	defer srv.Close()

	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Probe: true}).fetchAccountUsage(t.Name(), fakeJWT, refresh)
	if !errors.Is(err, errAccountMismatch) || len(got) != 0 || usageRequests != 1 {
		t.Fatalf("refreshed session crossed account boundary: rows=%d requests=%d err=%v", len(got), usageRequests, err)
	}
}

func TestScheduledRecoveryMeasuresCurrentBeforeFailedHistory(t *testing.T) {
	now := time.Date(2026, 11, 16, 10, 32, 0, 0, time.FixedZone("CST", 8*60*60))
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		start := time.UnixMilli(q.Start).UTC().Format("2006-01-02")
		starts = append(starts, start)
		if start == "2026-11-01" {
			fmt.Fprint(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"2026-11-15T23:58:00Z","model":"gpt-5","tokenUsage":{"inputTokens":27,"outputTokens":3}}]}`)
			return
		}
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer srv.Close()

	a := Adapter{Scheduled: true, HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }}
	got, err := a.fetchUsageWithToken(t.Name(), fakeJWT)
	if len(got) != 1 || got[0].Miss+got[0].Output != 30 || CollectionFailure(err) != "SOURCE_UNAVAILABLE" {
		t.Fatalf("current month did not survive historical failure: events=%+v err=%v", got, err)
	}
	if len(starts) < 2 || len(starts) > 3 || starts[0] != "2026-11-01" {
		t.Fatalf("unbounded or misordered UTC recovery: %v", starts)
	}
	covered := TakeAccountCoverage(t.Name())
	if len(covered) != 1 || covered[0].From != "2026-11-01" || covered[0].To != "2026-11-16" || covered[0].Audit.Validation != "open" {
		t.Fatalf("incorrect open/closed coverage: %+v", covered)
	}
	lookback, ok := TakeLookback(t.Name())
	if !ok || lookback.From >= covered[0].From {
		t.Fatalf("selecting the recent batch hid older gaps: %+v", lookback)
	}
}

func TestScheduledRecoveryDoesNotStarveOlderMissingMonths(t *testing.T) {
	now := time.Date(2026, 11, 16, 6, 0, 0, 0, time.UTC)
	months := map[string]bool{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var q struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&q)
		month := time.UnixMilli(q.Start).UTC().Format("2006-01")
		if month != "2026-11" && month != "2026-10" {
			months[month] = true
		}
		if month != "2026-11" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"totalUsageEventsCount":0,"usageEventsDisplay":[]}`)
	}))
	defer srv.Close()

	a := Adapter{Scheduled: true, HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }}
	for i := 0; i < 7; i++ {
		_, _ = a.fetchUsageWithToken(t.Name(), fakeJWT)
		TakeAccountCoverage(t.Name())
		TakeLookback(t.Name())
		now = now.AddDate(0, 0, 1)
	}
	if len(months) < 5 {
		t.Fatalf("same old windows starved the rest: %v", months)
	}
}

func TestScheduledRecoveryRejectsFilteredEventsOutsideWindow(t *testing.T) {
	now := time.Date(2026, 11, 16, 2, 32, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != filteredRPC {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		fmt.Fprint(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"2026-10-30T23:58:00Z","model":"gpt-5","tokenUsage":{"inputTokens":27,"outputTokens":3}}]}`)
	}))
	defer srv.Close()
	a := Adapter{Scheduled: true, Probe: true, HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }}
	got, err := a.fetchUsageWithToken(t.Name(), billingTestJWT())
	if err == nil || len(got) != 0 || len(TakeAccountCoverage(t.Name())) != 0 {
		t.Fatalf("out-of-window event became a fresh observation: rows=%d err=%v", len(got), err)
	}
}

func TestFilteredMalformedTokenMeterDoesNotBecomeZero(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"2026-11-15T23:58:00Z","model":"gpt-5","tokenUsage":{"inputTokens":"unknown"}}]}`)
	}))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchFiltered(fakeJWT, 1, 2)
	if !errors.Is(err, errSchemaChanged) || len(got.events) != 0 {
		t.Fatalf("unknown token meter became a measurement: rows=%d err=%v", len(got.events), err)
	}
}

func TestBillingConflictingDuplicateInvalidatesWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		meter, next := 10, `"next"`
		if r.URL.Query().Get("cursor") == "next" {
			meter, next = 20, "null"
		}
		fmt.Fprintf(w, `{"data":[{"billingItemId":"same-id","eventTime":"2026-11-01T03:43:46Z","hasTokenMeters":true,"principal":{"type":"user","id":"user_test"},"usage":{"prompt-text-tokens":"%d"}}],"pagination":{"nextCursor":%s}}`, meter, next)
	}))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchBillingWindow(billingTestJWT(), billingTestSpan())
	if CollectionFailure(err) != "COVERAGE" || len(got.events) != 0 {
		t.Fatalf("conflicting pages produced a measurement: rows=%d err=%v", len(got.events), err)
	}
}

func TestBillingDiagnosticsDoNotLogUntrustedSchemaFields(t *testing.T) {
	const untrusted = "privateUserIdentifier"
	for _, body := range []string{
		`{"availability":"` + untrusted + `"}`,
		`{"availability":{"` + untrusted + `":true}}`,
		`{"` + untrusted + `":"secret"}`,
	} {
		_, _, _, err := decodeBillingPage([]byte(body), "user_test", billingTestSpan(), billingTestSpan().start)
		if err == nil || strings.Contains(err.Error(), untrusted) {
			t.Fatalf("schema diagnostic echoed untrusted fields: %v", err)
		}
	}
}

func TestFilteredMixedUnknownMetersLeaveTheWindowUnverified(t *testing.T) {
	raw := []byte(`{"totalUsageEventsCount":2,"usageEventsDisplay":[
		{"timestamp":"2026-11-15T23:58:00Z","tokenUsage":{"inputTokens":27}},
		{"timestamp":"2026-11-15T23:59:00Z","tokenUsage":{"renamedInputTokens":32}}
	]}`)
	if _, _, _, _, err := parseFiltered(raw); CollectionFailure(err) != "SCHEMA" {
		t.Fatalf("mixed meters became a measurement: %v", err)
	}
}

func TestFilteredUsesProviderEventIdentity(t *testing.T) {
	raw := []byte(`{"totalUsageEventsCount":2,"usageEventsDisplay":[
		{"usageEventId":"test-one","timestamp":"2026-11-15T23:58:00Z","tokenUsage":{"inputTokens":27}},
		{"usageEventId":"test-one","timestamp":"2026-11-15T23:58:00Z","tokenUsage":{"inputTokens":27}}
	]}`)
	events, _, _, _, err := parseFiltered(raw)
	if err != nil || len(events) != 2 || events[0].RequestID != events[1].RequestID {
		t.Fatalf("repeat changed observation identity: %v", err)
	}
}

func TestFilteredProviderIdentityIsStableWithoutCopyingRawFields(t *testing.T) {
	const opaqueID = "/fixture/private/path/provider-id"
	row := func(id string) []byte {
		return []byte(fmt.Sprintf(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"requestId":%q,"tokenUsage":{"inputTokens":10}}]}`, id))
	}
	first, _, _, _, firstErr := parseFiltered(row(opaqueID))
	replay, _, _, _, replayErr := parseFiltered(row(opaqueID))
	other, _, _, _, otherErr := parseFiltered(row(opaqueID + "-other"))
	if firstErr != nil || replayErr != nil || otherErr != nil || len(first) != 1 || len(replay) != 1 || len(other) != 1 {
		t.Fatal("provider row did not parse")
	}
	if strings.Contains(first[0].RequestID, opaqueID) || first[0].RequestID == "" ||
		first[0].RequestID != replay[0].RequestID || first[0].RequestID == other[0].RequestID {
		t.Fatal("normalized request ID lost privacy or stable identity")
	}
}
