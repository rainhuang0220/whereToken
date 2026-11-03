package scan

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

// The fixture JWT payload is {"sub":"test-user"}. The other payload is
// {"sub":"other-user"}. Neither value is a live session.
const windowJWT = "eyJhbGciOiJub25lIn0.eyJzdWIiOiJ0ZXN0LXVzZXIifQ.sig"
const otherJWT = "eyJhbGciOiJub25lIn0.eyJzdWIiOiJvdGhlci11c2VyIn0.sig"

func TestCoveredMonthKeepsEarlierObservationAndBlocksAnotherAccount(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedChainObservation(t, testhome.New(home))
	bindCursorSubject(t, testhome.New(home), "test-user")

	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	stamp := when.UnixMilli()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if r.Header.Get("Authorization") != "Bearer "+windowJWT {
			http.Error(w, "no", http.StatusUnauthorized)
			return
		}
		if !strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"win","tokenUsage":{"inputTokens":7,"outputTokens":1,"cacheReadTokens":1,"cacheWriteTokens":1}}]}`, stamp)
	}))
	t.Cleanup(srv.Close)

	res := Run(testhome.New(home), []adapter.Adapter{cursor.Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Probe:   true,
		Now:     func() time.Time { return when },
		Unseal:  func(string) (string, error) { return windowJWT, nil },
	}})
	if len(res.Errors) != 0 {
		t.Fatalf("errors=%v", res.Errors)
	}
	if hits != 1 {
		t.Fatalf("hits=%d", hits)
	}
	var fresh, retained event.UsageEvent
	var freshN, retainedN int
	for _, e := range res.Events {
		if e.Source != "cursor" {
			continue
		}
		switch e.Derivation {
		case event.DeriveProviderAPI:
			fresh = e
			freshN++
		case event.DeriveRetained:
			retained = e
			retainedN++
		}
	}
	if freshN != 1 || retainedN != 1 {
		t.Fatalf("fresh=%d retained=%d", freshN, retainedN)
	}
	if fresh.Miss != 7 || fresh.CacheRead != 1 || fresh.CacheCreate != 1 || fresh.Output != 1 {
		t.Fatalf("fresh=%+v", fresh)
	}
	if fresh.Timestamp.UTC().Format("2006-01-02") != "2026-11-02" {
		t.Fatalf("fresh day=%s", fresh.Timestamp.UTC())
	}
	if retained.Miss != chainObserved || retained.Timestamp.UTC().Format("2006-01-02") != "2026-10-23" {
		t.Fatalf("retained=%+v", retained)
	}
	if tokenSum(res.Events, "cursor") != chainObserved+10 {
		t.Fatalf("cursor tokens=%d", tokenSum(res.Events, "cursor"))
	}

	hits = 0
	again := Run(testhome.New(home), []adapter.Adapter{cursor.Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Probe:   true,
		Now:     func() time.Time { return when },
		Unseal:  func(string) (string, error) { return otherJWT, nil },
	}})
	joined := strings.Join(again.Errors, "\n")
	if !strings.Contains(joined, cursor.KindAccountMismatch) || strings.Contains(strings.ToLower(joined), "auth") {
		t.Fatalf("errors=%v", again.Errors)
	}
	if hits != 0 {
		t.Fatalf("mismatch reached the API: hits=%d", hits)
	}
	if tokenSum(again.Events, "cursor") == chainObserved {
		t.Fatal("a different account reused the stored observation")
	}

	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	sub, err := lg.Meta(cursor.AccountSubjectMeta)
	if err != nil || sub != "test-user" {
		t.Fatalf("subject=%q err=%v", sub, err)
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || tokenSum(chained, "cursor") != chainObserved+10 {
		t.Fatalf("ok=%v err=%v sum=%d", ok, err, tokenSum(chained, "cursor"))
	}
	var sawOld bool
	for _, e := range chained {
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-10-23" && e.Miss == chainObserved {
			sawOld = true
		}
	}
	if !sawOld {
		t.Fatal("the 2026-10-23 observation left the chain")
	}
}

// An observation stored before any account subject exists must not be
// given to whichever Cursor account connects first. The synthetic JWTs
// are not live sessions.
func TestUnboundObservationIsNotClaimedByTheFirstAccount(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedChainObservation(t, testhome.New(home))

	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	stamp := when.UnixMilli()
	var hits int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"win","tokenUsage":{"inputTokens":7,"outputTokens":1,"cacheReadTokens":1,"cacheWriteTokens":1}}]}`, stamp)
	}))
	t.Cleanup(srv.Close)

	runAs := func(token string) Result {
		t.Helper()
		return Run(testhome.New(home), []adapter.Adapter{cursor.Adapter{
			HTTP:    srv.Client(),
			APIBase: srv.URL,
			Probe:   true,
			Now:     func() time.Time { return when },
			Unseal:  func(string) (string, error) { return token, nil },
		}})
	}
	for _, token := range []string{otherJWT, windowJWT} {
		hits = 0
		got := runAs(token)
		joined := strings.Join(got.Errors, "\n")
		if !strings.Contains(joined, "UNBOUND_HISTORY") || strings.Contains(strings.ToLower(joined), "auth") {
			t.Fatalf("token subject scan errors=%v", got.Errors)
		}
		if strings.Contains(joined, "other-user") || strings.Contains(joined, "test-user") || strings.Contains(joined, "eyJ") {
			t.Fatalf("error carried account material: %v", got.Errors)
		}
		if hits != 0 {
			t.Fatalf("unbound history reached the API: hits=%d", hits)
		}
		for _, e := range got.Events {
			if e.Source == "cursor" && e.Derivation == event.DeriveProviderAPI {
				t.Fatalf("unbound history accepted a new account window: %+v", e)
			}
		}
		if tokenSum(got.Events, "cursor") == chainObserved+10 {
			t.Fatal("the first account was merged onto the unbound observation")
		}
	}

	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	sub, err := lg.Meta(cursor.AccountSubjectMeta)
	if err != nil || sub != "" {
		t.Fatalf("subject=%q err=%v", sub, err)
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || tokenSum(chained, "cursor") != chainObserved || len(chained) != 1 {
		t.Fatalf("ok=%v err=%v days=%d sum=%d", ok, err, len(chained), tokenSum(chained, "cursor"))
	}
	if chained[0].Timestamp.UTC().Format("2006-01-02") != "2026-10-23" || chained[0].Miss != chainObserved {
		t.Fatalf("observation changed: %+v", chained[0])
	}
}

func bindCursorSubject(t *testing.T, home adapter.Home, sub string) {
	t.Helper()
	p, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	if err := lg.SetMeta(cursor.AccountSubjectMeta, sub); err != nil {
		t.Fatal(err)
	}
}

func TestHourlyScanKeepsAnEarlyDayAndSkipsCoveredMonths(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	early := event.UsageEvent{
		Source: "cursor", RequestID: "day-2024-06-01",
		Timestamp: time.Date(2024, 6, 1, 12, 0, 0, 0, time.UTC),
		Miss:      41, Quality: event.QualityAuthoritative,
		Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC), []event.UsageEvent{early}); err != nil {
		t.Fatal(err)
	}
	if err := lg.SetMeta(cursor.AccountSubjectMeta, "test-user"); err != nil {
		t.Fatal(err)
	}
	lg.Close()

	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		day := time.UnixMilli(body.Start).UTC().Format("2006-01-02")
		starts = append(starts, day)
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"%s","tokenUsage":{"inputTokens":1}}]}`, body.Start+int64(12*time.Hour/time.Millisecond), day)
	}))
	t.Cleanup(srv.Close)
	ad := func(when time.Time) cursor.Adapter {
		return cursor.Adapter{
			HTTP: srv.Client(), APIBase: srv.URL,
			Now:    func() time.Time { return when },
			Unseal: func(string) (string, error) { return windowJWT, nil },
		}
	}
	first := Run(testhome.New(home), []adapter.Adapter{ad(time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC))})
	if len(first.Errors) != 0 {
		t.Fatalf("first errors=%v", first.Errors)
	}
	firstN := len(starts)
	if firstN < 10 {
		t.Fatalf("first starts=%d %v", firstN, starts)
	}
	starts = nil
	second := Run(testhome.New(home), []adapter.Adapter{ad(time.Date(2027, 11, 2, 15, 0, 0, 0, time.UTC))})
	if len(second.Errors) != 0 {
		t.Fatalf("second errors=%v starts=%v", second.Errors, starts)
	}
	joined := strings.Join(starts, ",")
	if strings.Contains(joined, "2026-08-01") || strings.Contains(joined, "2026-10-27") {
		t.Fatalf("covered month was downloaded again: %v", starts)
	}
	if !strings.Contains(joined, "2027-11-01") || !strings.Contains(joined, "2026-11-01") {
		t.Fatalf("gap or open month missing: %v", starts)
	}
	if len(starts) >= firstN {
		t.Fatalf("second pass %d was not smaller than first pass %d", len(starts), firstN)
	}
	lg, err = ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok {
		t.Fatalf("ok=%v err=%v", ok, err)
	}
	var kept int64
	for _, e := range chained {
		if e.Timestamp.UTC().Format("2006-01-02") == "2024-06-01" {
			kept += e.Miss
		}
	}
	if kept != 41 {
		t.Fatalf("early day=%d chain_days=%d", kept, len(chained))
	}
	var august int64
	for _, e := range chained {
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-08-01" {
			august += e.Miss
		}
	}
	if august == 0 {
		t.Fatalf("August 2026 left the chain after the lookback moved")
	}
}

func TestRetainedSnapshotDoesNotSkipTheBackfill(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	wide := []event.UsageEvent{
		{
			Source: "cursor", RequestID: "day-2025-11-01",
			Timestamp: time.Date(2025, 11, 1, 12, 0, 0, 0, time.UTC),
			Miss:      5, Quality: event.QualityAuthoritative,
			Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
		{
			Source: "cursor", RequestID: "day-2026-06-01",
			Timestamp: time.Date(2026, 6, 1, 12, 0, 0, 0, time.UTC),
			Miss:      6, Quality: event.QualityAuthoritative,
			Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
	}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), wide); err != nil {
		t.Fatal(err)
	}
	if err := lg.SetMeta(cursor.AccountSubjectMeta, "test-user"); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		day := time.UnixMilli(body.Start).UTC().Format("2006-01-02")
		starts = append(starts, day)
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"%s","tokenUsage":{"inputTokens":1}}]}`, body.Start, day)
	}))
	t.Cleanup(srv.Close)
	res := Run(testhome.New(home), []adapter.Adapter{cursor.Adapter{
		HTTP: srv.Client(), APIBase: srv.URL,
		Now:    func() time.Time { return time.Date(2026, 7, 15, 12, 0, 0, 0, time.UTC) },
		Unseal: func(string) (string, error) { return windowJWT, nil },
	}})
	if len(res.Errors) != 0 {
		t.Fatalf("errors=%v starts=%v", res.Errors, starts)
	}
	if !strings.Contains(strings.Join(starts, ","), "2025-12-01") {
		t.Fatalf("retained span skipped December 2025: %v", starts)
	}
}

func TestSameLookbackRefreshReplaysACoveredMonth(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		day := time.UnixMilli(body.Start).UTC().Format("2006-01-02")
		starts = append(starts, day)
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"%s","tokenUsage":{"inputTokens":1}}]}`, body.Start+int64(12*time.Hour/time.Millisecond), day)
	}))
	t.Cleanup(srv.Close)
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	ad := func(at time.Time) cursor.Adapter {
		return cursor.Adapter{
			HTTP: srv.Client(), APIBase: srv.URL,
			Now:    func() time.Time { return at },
			Unseal: func(string) (string, error) { return windowJWT, nil },
		}
	}
	first := Run(testhome.New(home), []adapter.Adapter{ad(when)})
	if len(first.Errors) != 0 {
		t.Fatalf("first errors=%v", first.Errors)
	}
	starts = nil
	second := Run(testhome.New(home), []adapter.Adapter{ad(when.Add(time.Hour))})
	if len(second.Errors) != 0 {
		t.Fatalf("second errors=%v starts=%v", second.Errors, starts)
	}
	joined := strings.Join(starts, ",")
	if strings.Contains(joined, "2026-08-01") {
		t.Fatalf("covered August was downloaded again: %v", starts)
	}
	if !strings.Contains(joined, "2026-10-01") || !strings.Contains(joined, "2026-11-01") {
		t.Fatalf("open edge missing: %v", starts)
	}
	var august event.UsageEvent
	var saw bool
	var retained int
	for _, e := range second.Events {
		if e.Source != "cursor" {
			continue
		}
		if e.Derivation == event.DeriveRetained {
			retained++
		}
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-08-01" && e.Miss > 0 {
			august = e
			saw = true
		}
	}
	if !saw || august.Derivation != event.DeriveProviderAPI || august.Quality != event.QualityAuthoritative {
		t.Fatalf("replay=%+v saw=%v", august, saw)
	}
	if retained != 0 {
		t.Fatalf("complete lookback kept %d retained rows", retained)
	}
}

func TestRateLimitKeepsVerifiedDaysAndRetriesTheMonth(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	phase := 0
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			Start int64 `json:"startDate"`
		}
		_ = json.NewDecoder(r.Body).Decode(&body)
		start := time.UnixMilli(body.Start).UTC()
		day := start.Format("2006-01-02")
		starts = append(starts, day)
		if phase == 1 && start.Year() == 2026 && start.Month() == time.March {
			w.Header().Set("Retry-After", "3600")
			http.Error(w, "later", http.StatusTooManyRequests)
			return
		}
		tokens := 5
		if phase == 2 && start.Year() == 2026 && start.Month() == time.March {
			tokens = 9
		}
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"%s","tokenUsage":{"inputTokens":%d,"outputTokens":1,"cacheReadTokens":1,"cacheWriteTokens":1}}]}`, body.Start, day, tokens)
	}))
	t.Cleanup(srv.Close)
	when := time.Date(2026, 3, 15, 12, 0, 0, 0, time.UTC)
	ad := cursor.Adapter{
		HTTP: srv.Client(), APIBase: srv.URL,
		Now:    func() time.Time { return when },
		Unseal: func(string) (string, error) { return windowJWT, nil },
	}
	run := func() Result {
		t.Helper()
		return Run(testhome.New(home), []adapter.Adapter{ad})
	}
	first := run()
	if len(first.Errors) != 0 {
		t.Fatalf("first errors=%v", first.Errors)
	}
	p, err := ledger.PathFor(testhome.New(home))
	if err != nil {
		t.Fatal(err)
	}
	dayTotal := func(day string) int64 {
		t.Helper()
		lg, err := ledger.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer lg.Close()
		chained, ok, err := lg.ChainedRetainedEvents("cursor")
		if err != nil || !ok {
			t.Fatalf("ok=%v err=%v", ok, err)
		}
		var n int64
		for _, e := range chained {
			if e.Timestamp.UTC().Format("2006-01-02") == day {
				n += e.Miss + e.CacheRead + e.CacheCreate + e.Output
			}
		}
		return n
	}
	auditOf := func(from, to string) ledger.WindowAudit {
		t.Helper()
		lg, err := ledger.Open(p)
		if err != nil {
			t.Fatal(err)
		}
		defer lg.Close()
		got, err := lg.WindowAudit("cursor", from, to)
		if err != nil {
			t.Fatal(err)
		}
		return got
	}
	if dayTotal("2026-01-01") != 8 || dayTotal("2026-03-01") != 8 {
		t.Fatalf("jan=%d march=%d", dayTotal("2026-01-01"), dayTotal("2026-03-01"))
	}
	january := auditOf("2026-01-01", "2026-01-31")
	if january.Validation != "ok" || january.Declared != 1 || january.RawRows != 1 || january.Deduped != 1 || january.Miss != 5 {
		t.Fatalf("january audit=%+v", january)
	}
	march := auditOf("2026-03-01", "2026-03-15")
	if march.Validation != "open" || march.Declared != 1 || march.Days != 1 {
		t.Fatalf("march audit=%+v", march)
	}
	absent := auditOf("2024-06-01", "2024-06-30")
	if absent.Validation != "missing" || absent.Miss != 0 {
		t.Fatalf("unaudited window=%+v", absent)
	}
	phase = 1
	starts = nil
	second := run()
	if len(second.Errors) == 0 {
		t.Fatal("rate limit returned no error")
	}
	if strings.Contains(strings.Join(starts, ","), "2026-01-01") || len(starts) > 3 {
		t.Fatalf("rate-limited refresh redownloaded history: %v", starts)
	}
	if dayTotal("2026-01-01") != 8 || dayTotal("2026-03-01") != 8 {
		t.Fatalf("rate limit rewrote verified days jan=%d march=%d", dayTotal("2026-01-01"), dayTotal("2026-03-01"))
	}
	if got := auditOf("2026-03-01", "2026-03-15"); got.Validation != "open" || got.Miss != 5 {
		t.Fatalf("rate limit replaced the march audit: %+v", got)
	}
	phase = 2
	starts = nil
	third := run()
	if len(third.Errors) != 0 {
		t.Fatalf("retry errors=%v starts=%v", third.Errors, starts)
	}
	if strings.Contains(strings.Join(starts, ","), "2026-01-01") {
		t.Fatalf("retry downloaded a covered month: %v", starts)
	}
	if !strings.Contains(strings.Join(starts, ","), "2026-03-01") {
		t.Fatalf("retry skipped the rate-limited month: %v", starts)
	}
	if dayTotal("2026-03-01") != 12 || dayTotal("2026-01-01") != 8 {
		t.Fatalf("retry jan=%d march=%d", dayTotal("2026-01-01"), dayTotal("2026-03-01"))
	}
	if got := auditOf("2026-03-01", "2026-03-15"); got.Validation != "open" || got.Miss != 9 || got.Declared != 1 {
		t.Fatalf("retried march audit=%+v", got)
	}
}

// A legacy verified snapshot is retained while the late-2026 dashboard v2
// provides new events. This checks the real scan->ledger join, not only the
// adapter parser.
func TestBillingV2ScanPreserves166DayHistoryAndAddsRecentTokens(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	sub := "auth0|user_test"
	bindCursorSubject(t, testhome.New(home), sub)
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"auth0|user_test"}`)) + ".sig"
	var starts []string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			io.WriteString(w, `{}`)
			return
		}
		if r.URL.Path != "/api/dashboard/v2/query-billing-items" {
			http.NotFound(w, r)
			return
		}
		st := r.URL.Query().Get("startTime")
		starts = append(starts, st)
		end := r.URL.Query().Get("endTime")
		if strings.HasPrefix(st, "2026-10-01") && !strings.HasPrefix(end, "2026-10-31T23:59:59Z") {
			t.Errorf("October request must use the known-good UTC-second boundary: %s", end)
		}
		ts, err := time.Parse(time.RFC3339Nano, st)
		if err != nil {
			t.Errorf("bad start: %v", err)
			return
		}
		// Include an October 22 event that must be parsed as part of the
		// full-month page, but never re-attributed over verified history.
		if ts.Month() == time.October {
			fmt.Fprint(w, `{"data":[{"billingItemId":"before-baseline","eventTime":"2026-10-22T12:00:00Z","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"999"},"totalTokens":"999"},{"billingItemId":"after-baseline","eventTime":"2026-10-25T12:00:00Z","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"5","cached-prompt-text-tokens":"2","cache-write-text-tokens":"1","completion-text-tokens":"2"},"totalTokens":"10"}],"pagination":{"nextCursor":null}}`)
			return
		}
		fmt.Fprintf(w, `{"data":[{"billingItemId":"win-%s","eventTime":"%s","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"5","cached-prompt-text-tokens":"2","cache-write-text-tokens":"1","completion-text-tokens":"2"},"totalTokens":"10"}],"pagination":{"nextCursor":null}}`, ts.Format("20060102"), ts.Add(time.Hour).Add(func() time.Duration {
			if ts.Month() == time.October {
				return 24 * 24 * time.Hour
			}
			return 0
		}()).Format(time.RFC3339Nano))
	}))
	defer srv.Close()
	ad := cursor.Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }, Unseal: func(string) (string, error) { return jwt, nil }}
	res := Run(testhome.New(home), []adapter.Adapter{ad})
	if len(res.Errors) > 0 {
		t.Fatalf("unexpected errors: %v", res.Errors)
	}
	if len(starts) != 2 || !strings.HasPrefix(starts[0], "2026-10-01") || !strings.HasPrefix(starts[1], "2026-11-01") {
		t.Fatalf("wrong windows: %v", starts)
	}
	if got := tokenSum(res.Events, "cursor"); got != chainObserved+20 {
		t.Fatalf("Cursor total=%d want=%d", got, chainObserved+20)
	}
	lg := openHistoryLedger(t, testhome.New(home))
	defer lg.Close()
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok || tokenSum(chained, "cursor") != chainObserved+20 {
		t.Fatalf("persisted chain sum=%d err=%v", tokenSum(chained, "cursor"), err)
	}
	if last, err := lg.VerifiedSnapshotLastDay("cursor"); err != nil || last != "2026-10-23" {
		t.Fatalf("baseline last=%s err=%v", last, err)
	}
	audit, err := lg.WindowAudit("cursor", "2026-11-01", "2026-11-03")
	if err != nil || audit.Validation != "open" || audit.Declared != -1 {
		t.Fatalf("v2 audit=%+v err=%v", audit, err)
	}
}

func TestBillingAvailabilityLeavesHistoryUnknownButShowsRecentUsage(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	bindCursorSubject(t, testhome.New(home), "auth0|user_test")
	now := time.Date(2026, 11, 3, 12, 0, 0, 0, time.UTC)
	jwt := "eyJhbGciOiJub25lIn0." + base64.RawURLEncoding.EncodeToString([]byte(`{"sub":"auth0|user_test"}`)) + ".sig"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			fmt.Fprint(w, `{}`)
			return
		}
		if strings.HasPrefix(r.URL.Query().Get("startTime"), "2026-11-01") {
			fmt.Fprint(w, `{"data":[{"billingItemId":"current","eventTime":"2026-11-02T12:00:00Z","principal":{"type":"user","id":"user_test"},"hasTokenMeters":true,"usage":{"prompt-text-tokens":"10"},"totalTokens":"10"}],"pagination":{"nextCursor":null}}`)
			return
		}
		fmt.Fprint(w, `{"availability":{}}`)
	}))
	defer srv.Close()
	a := cursor.Adapter{HTTP: srv.Client(), APIBase: srv.URL, Now: func() time.Time { return now }, Unseal: func(string) (string, error) { return jwt, nil }}
	for n := 0; n < 2; n++ {
		res := Run(testhome.New(home), []adapter.Adapter{a})
		if len(res.Errors) == 0 {
			t.Fatalf("missing historical range must show partial notice")
		}
		if got := tokenSum(res.Events, "cursor"); got != chainObserved+10 {
			t.Fatalf("pass %d total=%d want=%d", n, got, chainObserved+10)
		}
		lg := openHistoryLedger(t, testhome.New(home))
		chained, ok, err := lg.ChainedRetainedEvents("cursor")
		if err != nil || !ok || tokenSum(chained, "cursor") != chainObserved+10 {
			t.Fatalf("pass %d persisted history=%d err=%v", n, tokenSum(chained, "cursor"), err)
		}
		audit, err := lg.WindowAudit("cursor", "2026-11-01", "2026-11-03")
		if err != nil || audit.Validation != "open" {
			t.Fatalf("pass %d good month audit=%+v err=%v", n, audit, err)
		}
		oct, err := lg.WindowAudit("cursor", "2026-10-24", "2026-10-31")
		if err != nil || oct.Validation != "missing" {
			t.Fatalf("pass %d unavailable month was measured %+v err=%v", n, oct, err)
		}
		lg.Close()
	}
}
