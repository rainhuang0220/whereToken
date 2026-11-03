package cursor

import (
	"encoding/base64"
	"fmt"
	"github.com/rainhuang0220/whereToken/internal/event"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func billingTestJWT() string {
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"auth0|user_test"}`))
	return "eyJhbGciOiJub25lIn0." + claims + ".sig"
}

func billingTestSpan() usageSpan {
	s := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC)
	return usageSpan{start: s, end: s.AddDate(0, 0, 2), inclusive: false}
}

func TestBillingV2ParsesFourMetersAndCursorPagination(t *testing.T) {
	pages := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != billingV2Path || r.URL.Query().Get("scope") != "user" || r.Method != "GET" {
			t.Errorf("wrong endpoint %s", r.URL.Path)
			w.WriteHeader(400)
			return
		}
		if !strings.Contains(r.Header.Get("Cookie"), "WorkosCursorSessionToken=") {
			t.Errorf("missing cookie")
		}
		pages++
		if pages == 1 {
			if r.URL.Query().Get("cursor") != "" {
				t.Errorf("unexpected initial cursor")
			}
			io.WriteString(w, `{"data":[{"billingItemId":"one","eventTime":"2026-11-01T03:43:46.022Z","hasTokenMeters":true,"principal":{"type":"user","id":"user_test"},"usage":{"cache-write-text-tokens":"0","cached-prompt-text-tokens":"46400","completion-text-tokens":"312","prompt-text-tokens":"1263"},"totalTokens":"47975","displayExtras":{"model":"composer-2.5"}}],"pagination":{"nextCursor":"abc"}}`)
			return
		}
		if r.URL.Query().Get("cursor") != "abc" {
			t.Errorf("wrong followup cursor")
		}
		io.WriteString(w, `{"data":[{"billingItemId":"two","eventTime":"2026-11-02T02:00:00Z","hasTokenMeters":true,"principal":{"type":"user","id":"user_test"},"usage":{"prompt-text-tokens":"1","cached-prompt-text-tokens":"2","cache-write-text-tokens":"3","completion-text-tokens":"4"},"totalTokens":"10","displayExtras":{"model":"test"}}],"pagination":{"nextCursor":null}}`)
	}))
	defer srv.Close()
	got, err := (Adapter{APIBase: srv.URL, HTTP: srv.Client()}).fetchBillingWindow(billingTestJWT(), billingTestSpan())
	if err != nil {
		t.Fatal(err)
	}
	if pages != 2 || got.raw != 2 || got.declared != -1 || len(got.events) != 2 {
		t.Fatalf("pages=%d got=%+v", pages, got)
	}
	first := got.events[0]
	if first.Miss != 1263 || first.CacheRead != 46400 || first.CacheCreate != 0 || first.Output != 312 {
		t.Fatalf("wrong meter mapping %+v", first)
	}
	if !strings.HasPrefix(first.RequestID, "cursor-billing-v2:") {
		t.Fatal("missing stable item id")
	}
}

func TestBillingV2FailsClosedOnMissingDataAndTotals(t *testing.T) {
	cases := []struct{ name, body string }{
		{"missing-data", `{}`},
		{"missing-meter", `{"data":[{"billingItemId":"x","eventTime":"2026-11-01T03:43:46Z","hasTokenMeters":true,"usage":{"other-tokens":"10"}}],"pagination":{}}`},
		{"wrong-total", `{"data":[{"billingItemId":"x","eventTime":"2026-11-01T03:43:46Z","hasTokenMeters":true,"usage":{"prompt-text-tokens":"10"},"totalTokens":"20"}],"pagination":{}}`},
		{"missing-pagination", `{"data":[]}`},
		{"missing-next-cursor", `{"data":[],"pagination":{}}`},
		{"wrong-user", `{"data":[{"billingItemId":"x","eventTime":"2026-11-01T03:43:46Z","hasTokenMeters":true,"principal":{"type":"user","id":"user_other"},"usage":{"prompt-text-tokens":"10"}}],"pagination":{}}`},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { fmt.Fprint(w, tc.body) }))
			defer srv.Close()
			got, err := (Adapter{APIBase: srv.URL, HTTP: srv.Client()}).fetchBillingWindow(billingTestJWT(), billingTestSpan())
			if err == nil || len(got.events) > 0 {
				t.Fatalf("should reject %s, events=%d err=%v", tc.name, len(got.events), err)
			}
		})
	}
}

func TestBillingV2FallbackFromOldEmptyObjectPreservesValidatedWindows(t *testing.T) {
	oldCalls, v2Calls := 0, 0
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			oldCalls++
			fmt.Fprint(w, `{}`)
			return
		}
		if r.URL.Path != billingV2Path {
			t.Errorf("unexpected %s", r.URL.Path)
			w.WriteHeader(404)
			return
		}
		v2Calls++
		start, err := time.Parse(time.RFC3339Nano, r.URL.Query().Get("startTime"))
		if err != nil {
			t.Errorf("start time %v", err)
			w.WriteHeader(400)
			return
		}
		eventTime := start.Add(time.Hour)
		if start.Format("2006-01") == "2025-10" {
			eventTime = start.Add(28 * 24 * time.Hour)
		}
		ts := eventTime.Format(time.RFC3339Nano)
		fmt.Fprintf(w, `{"data":[{"billingItemId":"month-%s","eventTime":"%s","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"5","cached-prompt-text-tokens":"10","cache-write-text-tokens":"3","completion-text-tokens":"2"},"totalTokens":"20"}],"pagination":{"nextCursor":null}}`, start.Format("2006-01"), ts)
	}))
	defer srv.Close()
	a := Adapter{APIBase: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }}
	evs, err := a.fetchUsageWithToken("billing-test-root", billingTestJWT())
	if err != nil {
		t.Fatal(err)
	}
	if oldCalls != 1 || v2Calls < 10 || len(evs) != v2Calls {
		t.Fatalf("legacy=%d v2=%d events=%d", oldCalls, v2Calls, len(evs))
	}
	cov := TakeAccountCoverage("billing-test-root")
	if len(cov) != v2Calls {
		t.Fatalf("coverage=%d windows=%d", len(cov), v2Calls)
	}
	for _, w := range cov {
		if w.Audit.Declared != -1 || w.Audit.RawRows != 1 || (w.Audit.Validation != "ok" && w.Audit.Validation != "open") {
			t.Fatalf("invalid audit %+v", w)
		}
	}
}

func TestBillingV2CannotReplaceOldVerifiedSnapshot(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	requests := []string{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			fmt.Fprint(w, `{}`)
			return
		}
		if r.URL.Path != billingV2Path {
			t.Errorf("wrong path")
			w.WriteHeader(404)
			return
		}
		from := r.URL.Query().Get("startTime")
		requests = append(requests, from)
		ts, err := time.Parse(time.RFC3339Nano, from)
		if err != nil {
			t.Fatal(err)
		}
		fmt.Fprintf(w, `{"data":[{"billingItemId":"test-%s","eventTime":"%s","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"7"},"totalTokens":"7"}],"pagination":{"nextCursor":null}}`, ts.Format("20060102"), ts.Add(time.Hour).Add(func() time.Duration {
			if ts.Month() == time.October {
				return 24 * 24 * time.Hour
			}
			return 0
		}()).Format(time.RFC3339Nano))
	}))
	defer srv.Close()
	a := Adapter{APIBase: srv.URL, HTTP: srv.Client(), Now: func() time.Time { return now }, ProtectedThrough: "2026-10-23"}
	evs, err := a.fetchUsageWithToken("billing-protected-root", billingTestJWT())
	if err != nil {
		t.Fatal(err)
	}
	if len(requests) != 2 || len(evs) != 2 {
		t.Fatalf("requests=%v events=%d", requests, len(evs))
	}
	if !strings.HasPrefix(requests[0], "2026-10-01") || !strings.HasPrefix(requests[1], "2026-11-01") {
		t.Fatalf("old verified date requested for overwrite: %v", requests)
	}
	ranges := TakeAccountCoverage("billing-protected-root")
	if len(ranges) != 2 || ranges[0].From != "2026-10-24" || ranges[0].To != "2026-10-31" {
		t.Fatalf("bad protected coverage %+v", ranges)
	}
}

func TestBillingAvailabilityIsUnknownNotMeasuredZero(t *testing.T) {
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			fmt.Fprint(w, `{}`)
			return
		}
		start := r.URL.Query().Get("startTime")
		if strings.HasPrefix(start, "2026-11-01") {
			fmt.Fprint(w, `{"data":[{"billingItemId":"recent","eventTime":"2026-11-02T12:00:00Z","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"10"},"totalTokens":"10"}],"pagination":{"nextCursor":null}}`)
		} else {
			fmt.Fprint(w, `{"availability":{}}`)
		}
	}))
	defer srv.Close()
	root := "billing-availability-unknown-test"
	got, err := (Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }}).fetchUsageWithToken(root, billingTestJWT())
	if err == nil || len(got) != 1 {
		t.Fatalf("missing months must remain partial: events=%d err=%v", len(got), err)
	}
	if got[0].Miss != 10 || got[0].Quality != event.QualityDegraded {
		t.Fatalf("recent event lost: %+v", got[0])
	}
	cov := TakeAccountCoverage(root)
	if len(cov) != 1 || cov[0].From != "2026-11-01" || cov[0].To != "2026-11-03" {
		t.Fatalf("unavailable month marked complete: %+v", cov)
	}
}
