package cursor

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestAttributionFetchRevisitsProtectedHistoryWithoutChangingNormalScan(t *testing.T) {
	var months []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			fmt.Fprint(w, `{}`)
			return
		}
		if r.URL.Path != billingV2Path {
			http.NotFound(w, r)
			return
		}
		month := r.URL.Query().Get("startTime")[:7]
		months = append(months, month)
		stamp := month + "-23T12:00:00Z"
		model := "grok-4.6"
		if month == "2026-11" {
			stamp = "2026-11-02T12:00:00Z"
			model = "composer-2.5"
		}
		fmt.Fprintf(w, `{"data":[{"billingItemId":%q,"eventTime":%q,"principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"1","cached-prompt-text-tokens":"2","cache-write-text-tokens":"3","completion-text-tokens":"4"},"totalTokens":"10","displayExtras":{"model":%q}}],"pagination":{"nextCursor":null}}`, month, stamp, model)
	}))
	defer srv.Close()
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL, ProtectedThrough: "2026-10-23", Covered: []CoverageWindow{{From: "2026-10-01", To: "2026-10-31"}}, Now: func() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }}
	got, err := a.fetchAttributionWindows(billingTestJWT(), "2026-10-22", "2026-11-03")
	if err != nil {
		t.Fatal(err)
	}
	if len(months) != 2 || months[0] != "2026-10" || months[1] != "2026-11" || len(got.Events) != 2 || len(got.Windows) != 2 {
		t.Fatalf("months=%v events=%d windows=%v", months, len(got.Events), got.Windows)
	}
	if got.Events[0].Vendor != "xai" || got.Events[1].Vendor != "cursor" {
		t.Fatalf("vendors=%s/%s", got.Events[0].Vendor, got.Events[1].Vendor)
	}
	if got.Events[0].Provenance != "cursor_billing_v2" {
		t.Fatal("billing endpoint provenance missing")
	}
	if got.Windows[0].From != "2026-10-22" || got.Windows[0].To != "2026-10-31" || got.Windows[1].To != "2026-11-03" {
		t.Fatalf("coverage=%v", got.Windows)
	}
}

func TestAttributionUsesAuthenticatedCSVAfterUnavailableBilling(t *testing.T) {
	csvCalls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case filteredRPC:
			fmt.Fprint(w, `{}`)
		case billingV2Path:
			fmt.Fprint(w, `{"availability":{"reason":"missing_team_id"}}`)
		case "/api/dashboard/teams":
			fmt.Fprint(w, `{"teams":[]}`)
		case dashboardCSVPath:
			csvCalls++
			fmt.Fprintln(w, "Date,Model,Input (w/ Cache Write),Input (w/o Cache Write),Cache Read,Output Tokens,Total Tokens")
			fmt.Fprintln(w, "2026-11-02T12:00:00Z,grok-4.7,3,1,2,4,10")
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }}).fetchAttributionWindows(billingTestJWT(), "2026-11-01", "2026-11-03")
	if err != nil || csvCalls != 1 || len(got.Events) != 1 || got.Events[0].Provenance != "cursor_dashboard_csv" || len(got.Windows) != 1 {
		t.Fatalf("csv=%d events=%d windows=%v err=%v", csvCalls, len(got.Events), got.Windows, err)
	}
}

func TestAttributionFetchKeepsOnlyCompletedMonthsOnPartialFailure(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			fmt.Fprint(w, `{}`)
			return
		}
		if strings.HasPrefix(r.URL.Query().Get("startTime"), "2026-10") {
			fmt.Fprint(w, `{"availability":{"from":"private"}}`)
			return
		}
		fmt.Fprint(w, `{"data":[{"billingItemId":"nov","eventTime":"2026-11-02T12:00:00Z","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"1"},"totalTokens":"1","displayExtras":{"model":"auto"}}],"pagination":{"nextCursor":null}}`)
	}))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }}).fetchAttributionWindows(billingTestJWT(), "2026-10-01", "2026-11-03")
	if err == nil || len(got.Windows) != 1 || got.Windows[0].From != "2026-11-01" || len(got.Events) != 1 || got.Events[0].Vendor != "unknown" {
		t.Fatalf("events=%d coverage=%v err=%v", len(got.Events), got.Windows, err)
	}
	if strings.Contains(err.Error(), "private") {
		t.Fatal("response value leaked")
	}
}

func TestAttributionRejectsRowsOutsideRequestedUTCWindow(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"2026-10-31T23:59:59Z","model":"grok-4.7","tokenUsage":{"inputTokens":10}}]}`)
	}))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return time.Date(2026, 11, 9, 0, 0, 0, 0, time.UTC) }}).fetchAttributionWindows(billingTestJWT(), "2026-11-01", "2026-11-03")
	if err == nil || len(got.Events) != 0 || len(got.Windows) != 0 {
		t.Fatalf("events=%d windows=%d err=%v", len(got.Events), len(got.Windows), err)
	}
}

func TestFilteredMissingTokenMetersStayUnavailable(t *testing.T) {
	raw := []byte(`{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"2026-11-01T12:00:00Z","model":"default"}]}`)
	events, n, _, _, err := parseFiltered(raw)
	if err != nil || n != 1 || len(events) != 0 {
		t.Fatalf("missing meters became usage: n=%d events=%d err=%v", n, len(events), err)
	}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { w.Write(raw) }))
	defer srv.Close()
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL}).fetchFiltered(billingTestJWT(), 1, 2)
	if err == nil || len(got.events) != 0 {
		t.Fatalf("missing meters became measured zero: %v", err)
	}
}
