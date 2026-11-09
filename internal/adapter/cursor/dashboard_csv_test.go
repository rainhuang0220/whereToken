package cursor

import (
	"bytes"
	"encoding/csv"
	"errors"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

var csvTestHeader = []string{"Date", "Cloud Agent ID", "Automation ID", "Kind", "Model", "Max Mode", "Input (w/ Cache Write)", "Input (w/o Cache Write)", "Cache Read", "Output Tokens", "Total Tokens", "Cost"}

func csvTestRow() []string {
	return []string{"2026-11-01T14:31:35.085Z", "private-agent", "private-automation", "Included", "composer-2.5", "private-max", "177", "2", "56462", "66", "56707", "private-cost"}
}

func csvTestBody(header []string, rows ...[]string) string {
	var body strings.Builder
	w := csv.NewWriter(&body)
	_ = w.Write(header)
	for _, row := range rows {
		_ = w.Write(row)
	}
	w.Flush()
	return body.String()
}

func TestDashboardCSVParsesIndependentMetersAndMissingUsage(t *testing.T) {
	missing := csvTestRow()
	missing[4] = "auto"
	for i := 6; i <= 10; i++ {
		missing[i] = "-"
	}
	unknown := csvTestRow()
	unknown[4] = "default"
	unknown[0] = "2026-11-02T02:00:00+08:00"
	got, err := decodeUsageCSV(strings.NewReader(csvTestBody(csvTestHeader, csvTestRow(), missing, unknown)), billingTestSpan())
	if err != nil {
		t.Fatal(err)
	}
	if got.raw != 3 || got.declared != -1 || len(got.events) != 2 {
		t.Fatalf("raw=%d declared=%d events=%d", got.raw, got.declared, len(got.events))
	}
	first := got.events[0]
	if first.Miss != 2 || first.CacheCreate != 177 || first.CacheRead != 56462 || first.Output != 66 {
		t.Fatalf("four independent CSV meters were misinterpreted: %+v", first)
	}
	if first.Source != "cursor" || first.Vendor != "cursor" || first.Model != "composer-2.5" || first.RequestID != "" || !first.SkipRequest || first.Derivation != event.DeriveProviderAPI || first.Provenance != "cursor_dashboard_csv" || first.Quality != event.QualityAuthoritative {
		t.Fatalf("CSV attribution=%+v", first)
	}
	if got.events[1].Vendor != "unknown" || !got.events[1].Timestamp.Equal(time.Date(2026, 11, 1, 18, 0, 0, 0, time.UTC)) {
		t.Fatalf("unknown model or UTC conversion=%+v", got.events[1])
	}
	for _, e := range got.events {
		if e.SessionID != "" || e.Provider != "" || e.Workspace != "" || e.SourceRoot != "" {
			t.Fatalf("private CSV metadata escaped: %+v", e)
		}
	}
}

func TestDashboardCSVRejectsUnverifiableRowsWithoutPartialEvents(t *testing.T) {
	for _, tc := range []struct {
		name   string
		mutate func([]string) []string
	}{
		{"partial meters", func(r []string) []string { r[7] = ""; return r }},
		{"missing total", func(r []string) []string { r[10] = ""; return r }},
		{"total mismatch", func(r []string) []string { r[10] = "56708"; return r }},
		{"negative", func(r []string) []string { r[7] = "-1"; return r }},
		{"fraction", func(r []string) []string { r[7] = "2.5"; return r }},
		{"overflow", func(r []string) []string { r[7] = "9223372036854775807"; return r }},
		{"no zone", func(r []string) []string { r[0] = "2026-11-01T14:31:35"; return r }},
		{"before range", func(r []string) []string { r[0] = "2026-10-31T23:59:59.999Z"; return r }},
		{"exclusive end", func(r []string) []string { r[0] = "2026-11-03T00:00:00Z"; return r }},
		{"truncated record", func(r []string) []string { return r[:9] }},
		{"unavailable with positive total", func(r []string) []string {
			for i := 6; i <= 9; i++ {
				r[i] = "-"
			}
			return r
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			got, err := decodeUsageCSV(strings.NewReader(csvTestBody(csvTestHeader, csvTestRow(), tc.mutate(csvTestRow()))), billingTestSpan())
			if err == nil || len(got.events) != 0 || got.raw != 0 {
				t.Fatalf("unverifiable window returned partial usage: raw=%d events=%d err=%v", got.raw, len(got.events), err)
			}
		})
	}
}

func TestDashboardCSVAllowsZeroUnavailableAndUnknownColumns(t *testing.T) {
	row := csvTestRow()
	for i := 6; i <= 10; i++ {
		row[i] = "0"
	}
	header := append(append([]string{}, csvTestHeader...), "private-new-field")
	row = append(row, "private-new-value")
	got, err := decodeUsageCSV(strings.NewReader(csvTestBody(header, row)), billingTestSpan())
	if err != nil || got.raw != 1 || len(got.events) != 1 || got.events[0].Miss != 0 {
		t.Fatalf("explicit measured zero err=%v raw=%d events=%d", err, got.raw, len(got.events))
	}
	for i := 6; i <= 10; i++ {
		row[i] = ""
	}
	got, err = decodeUsageCSV(strings.NewReader(csvTestBody(header, row)), billingTestSpan())
	if err != nil || got.raw != 1 || len(got.events) != 0 {
		t.Fatalf("unavailable row became measured zero err=%v raw=%d events=%d", err, got.raw, len(got.events))
	}
}

type csvFailReader struct{ io.Reader }

func (r csvFailReader) Read(p []byte) (int, error) {
	n, err := r.Reader.Read(p)
	if err == io.EOF {
		return n, errors.New("private-read-payload")
	}
	return n, err
}

func TestDashboardCSVRejectsIncompleteAndOversizeBodiesPrivately(t *testing.T) {
	valid := csvTestBody(csvTestHeader, csvTestRow())
	for _, reader := range []io.Reader{
		csvFailReader{strings.NewReader(valid)},
		strings.NewReader(valid + "\"private-malformed-payload"),
		bytes.NewReader(make([]byte, (64<<20)+1)),
	} {
		got, err := decodeUsageCSV(reader, billingTestSpan())
		if err == nil || len(got.events) != 0 || strings.Contains(err.Error(), "private-") {
			t.Fatalf("body rejection failed: events=%d err=%v", len(got.events), err)
		}
	}
	got, err := decodeUsageCSV(strings.NewReader("private-header,private-secret\n"), billingTestSpan())
	if err == nil || len(got.events) != 0 || strings.Contains(err.Error(), "private-") {
		t.Fatalf("header payload escaped: %v", err)
	}
}

func TestDashboardCSVFetchUsesBoundedAccountCookieAndNoRedirect(t *testing.T) {
	span := billingTestSpan()
	token := billingTestJWT()
	calls := 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls++
		q := r.URL.Query()
		if r.Method != http.MethodGet || r.URL.Path != "/api/dashboard/export-usage-events-csv" || q.Get("strategy") != "tokens" || q.Get("startDate") != strconv.FormatInt(span.start.UnixMilli(), 10) || q.Get("endDate") != strconv.FormatInt(span.end.UnixMilli()-1, 10) || len(q) != 3 {
			t.Errorf("wrong CSV request method/path/query")
		}
		cookie, err := r.Cookie("WorkosCursorSessionToken")
		if err != nil {
			t.Error("missing account cookie")
			return
		}
		identity, err := url.QueryUnescape(cookie.Value)
		if err != nil || identity != "user_test::"+token || r.Header.Get("Authorization") != "" {
			t.Error("wrong account identity or redundant authorization")
		}
		_, _ = io.WriteString(w, csvTestBody(csvTestHeader, csvTestRow()))
	}))
	defer srv.Close()
	client := srv.Client()
	got, err := (Adapter{APIBase: srv.URL, HTTP: client}).fetchCSVWindow(token, span)
	if err != nil || calls != 1 || len(got.events) != 1 {
		t.Fatalf("fetch err=%v calls=%d events=%d", err, calls, len(got.events))
	}
	if client.CheckRedirect != nil || client.Timeout != 0 {
		t.Fatal("shared HTTP client was mutated")
	}
	if _, err := (Adapter{APIBase: srv.URL, HTTP: client}).fetchCSVWindow("invalid-private-token", span); err == nil || calls != 1 {
		t.Fatal("invalid identity made a request")
	}
	leaked := 0
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { leaked++; w.WriteHeader(500) }))
	defer other.Close()
	redirect := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { http.Redirect(w, r, other.URL, http.StatusFound) }))
	defer redirect.Close()
	got, err = (Adapter{APIBase: redirect.URL, HTTP: redirect.Client()}).fetchCSVWindow(token, span)
	if err == nil || leaked != 0 || len(got.events) != 0 {
		t.Fatalf("redirect leaked account request: calls=%d err=%v", leaked, err)
	}
}

type csvTestTransport func(*http.Request) (*http.Response, error)

func (f csvTestTransport) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestDashboardCSVFetchTimeoutAndErrorsDoNotExposeSession(t *testing.T) {
	client := &http.Client{Transport: csvTestTransport(func(r *http.Request) (*http.Response, error) {
		deadline, ok := r.Context().Deadline()
		if !ok || time.Until(deadline) > 20*time.Second {
			t.Error("CSV request has no bounded client timeout")
		}
		return nil, errors.New("private-network-payload " + r.Header.Get("Cookie"))
	})}
	_, err := (Adapter{HTTP: client}).fetchCSVWindow(billingTestJWT(), billingTestSpan())
	if err == nil || strings.Contains(err.Error(), "private-") || strings.Contains(err.Error(), billingTestJWT()) || strings.Contains(err.Error(), "user_test") {
		t.Fatalf("unsafe transport error: %v", err)
	}
}
