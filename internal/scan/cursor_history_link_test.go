package scan

import (
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

// The verified 2025-10-21 vector is the published component split.
// 2025-10-22 through 2025-10-25 are absent on purpose.
const oct21Total int64 = 6_346_980

func TestUnboundHistoryConfirmThenCollectsFilteredUsage(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	srv, hits := historyLinkServer(t, when)

	before := runHistoryLink(t, home, srv, windowJWT, when)
	joined := strings.Join(before.Errors, "\n")
	if !strings.Contains(joined, cursor.KindUnboundHistory) || strings.Contains(strings.ToLower(joined), "auth") {
		t.Fatalf("errors=%v", before.Errors)
	}
	if *hits != 0 {
		t.Fatalf("unbound history reached the API: hits=%d", *hits)
	}
	assertCursorSubject(t, testhome.New(home), "")
	assertNoHistoryDecision(t, testhome.New(home))

	lg := openHistoryLedger(t, testhome.New(home))
	prompt, err := DescribeCursorHistory(lg, "test-user")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{
		fmt.Sprintf("历史 token：%d", chainObserved+oct21Total),
		"日期：2025-10-21..2026-10-23",
		"已记录日数：2",
		"当前账号：test-user",
		"没有足够证据把这份历史关联到当前账号",
	} {
		if !strings.Contains(prompt, want) {
			t.Fatalf("prompt missing %q\n%s", want, prompt)
		}
	}
	if strings.Contains(strings.ToLower(prompt), "auth") || strings.Contains(prompt, "eyJ") || strings.Contains(prompt, "/") {
		t.Fatalf("prompt leaked material:\n%s", prompt)
	}
	lg.Close()

	lg = openHistoryLedger(t, testhome.New(home))
	if err := ApplyCursorHistory(lg, "test-user", "yes", when); err != nil {
		t.Fatal(err)
	}
	base, at := historyAudit(t, lg)
	if err := ApplyCursorHistory(lg, "test-user", "yes", when.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	base2, at2 := historyAudit(t, lg)
	if base2 != base || at2 != at || at == "" || !strings.Contains(base, "2025-10-21") {
		t.Fatalf("repeat confirm rewrote the audit base=%q at=%q then %q %q", base, at, base2, at2)
	}
	if got, _ := lg.Meta(cursor.HistoryAuditSourceMeta); got != cursor.HistoryAuditConfirm {
		t.Fatalf("audit source=%q", got)
	}
	if err := ApplyCursorHistory(lg, "test-user", "no", when.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if got, _ := lg.Meta(cursor.AccountSubjectMeta); got != "test-user" {
		t.Fatalf("a later no unbound the account: %q", got)
	}
	if got, _ := lg.Meta(cursor.HistoryDecisionMeta); got != cursor.HistoryConfirmed {
		t.Fatalf("decision=%q", got)
	}
	base3, at3 := historyAudit(t, lg)
	if base3 != base || at3 != at {
		t.Fatalf("a later no rewrote the audit")
	}
	lg.Close()
	assertCursorSubject(t, testhome.New(home), "test-user")

	*hits = 0
	after := runHistoryLink(t, home, srv, windowJWT, when)
	assertHistoryLinkErrors(t, after.Errors)
	if *hits == 0 {
		t.Fatal("confirmed account made no filtered request")
	}
	got := chainDayTotals(t, testhome.New(home), "cursor")
	if got["2025-10-21"] != oct21Total || got["2026-10-23"] != chainObserved {
		t.Fatalf("history=%v", got)
	}
	if got["2026-11-02"] != 10 {
		t.Fatalf("new day=%v", got)
	}
	for _, day := range []string{"2025-10-22", "2025-10-23", "2025-10-24", "2025-10-25"} {
		if _, ok := got[day]; ok {
			t.Fatalf("unknown day %s was fabricated", day)
		}
	}

	*hits = 0
	other := runHistoryLink(t, home, srv, otherJWT, when)
	otherErr := strings.Join(other.Errors, "\n")
	if !strings.Contains(otherErr, cursor.KindAccountMismatch) || strings.Contains(strings.ToLower(otherErr), "auth") {
		t.Fatalf("errors=%v", other.Errors)
	}
	if *hits != 0 {
		t.Fatalf("other account reached the API: hits=%d", *hits)
	}
	if again := chainDayTotals(t, testhome.New(home), "cursor"); fmt.Sprint(again) != fmt.Sprint(got) {
		t.Fatalf("other account changed history %v", again)
	}
	assertCursorSubject(t, testhome.New(home), "test-user")
}

func TestDeclinedHistoryStaysSeparateFromTheNewAccount(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	srv, hits := historyLinkServer(t, when)
	legacy := chainDayTotals(t, testhome.New(home), "cursor")

	lg := openHistoryLedger(t, testhome.New(home))
	if err := ApplyCursorHistory(lg, "test-user", "no", when); err != nil {
		t.Fatal(err)
	}
	base, at := historyAudit(t, lg)
	if err := ApplyCursorHistory(lg, "test-user", "no", when.Add(time.Hour)); err != nil {
		t.Fatal(err)
	}
	base2, at2 := historyAudit(t, lg)
	if base2 != base || at2 != at {
		t.Fatalf("repeat decline rewrote the audit")
	}
	if err := ApplyCursorHistory(lg, "other-user", "yes", when); err == nil || !strings.Contains(err.Error(), cursor.KindHistoryHeld) {
		t.Fatalf("other account confirm err=%v", err)
	}
	lg.Close()
	assertCursorSubject(t, testhome.New(home), "")

	*hits = 0
	got := runHistoryLink(t, home, srv, windowJWT, when)
	assertHistoryLinkErrors(t, got.Errors)
	if *hits == 0 {
		t.Fatal("declined account made no filtered request")
	}
	if sum := tokenSum(got.Events, "cursor"); sum == 0 || sum >= chainObserved {
		t.Fatalf("declined scan mixed the old history into the new account: sum=%d", sum)
	}
	for _, e := range got.Events {
		if e.Miss == chainObserved {
			t.Fatal("declined scan returned the unbound observation")
		}
	}
	if again := chainDayTotals(t, testhome.New(home), "cursor"); fmt.Sprint(again) != fmt.Sprint(legacy) {
		t.Fatalf("decline changed the old chain %v", again)
	}
	ns := cursor.AccountNamespace("test-user")
	if !strings.HasPrefix(ns, "cursor#") || strings.Contains(ns, "test-user") {
		t.Fatalf("namespace=%s", ns)
	}
	fresh := chainDayTotals(t, testhome.New(home), ns)
	if fresh["2026-11-02"] != 10 || fresh["2026-10-23"] != 0 || fresh["2025-10-21"] != 0 {
		t.Fatalf("namespace days=%v", fresh)
	}

	*hits = 0
	other := runHistoryLink(t, home, srv, otherJWT, when)
	if !strings.Contains(strings.Join(other.Errors, "\n"), cursor.KindUnboundHistory) || *hits != 0 {
		t.Fatalf("hits=%d errors=%v", *hits, other.Errors)
	}
	if again := chainDayTotals(t, testhome.New(home), ns); fmt.Sprint(again) != fmt.Sprint(fresh) {
		t.Fatalf("other account wrote the namespace %v", again)
	}

	lg = openHistoryLedger(t, testhome.New(home))
	if err := ApplyCursorHistory(lg, "test-user", "yes", when.Add(2*time.Hour)); err != nil {
		t.Fatal(err)
	}
	base3, at3 := historyAudit(t, lg)
	if base3 != base || at3 == at {
		t.Fatalf("confirm after decline baseline=%q at=%q original at=%q", base3, at3, at)
	}
	if got, _ := lg.Meta(cursor.HistoryAuditSourceMeta); got != cursor.HistoryAuditConfirm {
		t.Fatalf("audit source=%q", got)
	}
	lg.Close()
	assertCursorSubject(t, testhome.New(home), "test-user")

	*hits = 0
	restored := runHistoryLink(t, home, srv, windowJWT, when)
	assertHistoryLinkErrors(t, restored.Errors)
	want := chainObserved + oct21Total + 10
	if sum := tokenSum(restored.Events, "cursor"); sum != want {
		t.Fatalf("restored sum=%d want=%d", sum, want)
	}
	restoredDays := chainDayTotals(t, testhome.New(home), "cursor")
	if restoredDays["2025-10-21"] != oct21Total || restoredDays["2026-10-23"] != chainObserved || restoredDays["2026-11-02"] != 10 {
		t.Fatalf("restored history=%v", restoredDays)
	}
	for _, day := range []string{"2025-10-22", "2025-10-23", "2025-10-24", "2025-10-25"} {
		if _, ok := restoredDays[day]; ok {
			t.Fatalf("unknown day %s was fabricated", day)
		}
	}
	if nsAgain := chainDayTotals(t, testhome.New(home), ns); nsAgain["2026-11-02"] != 10 {
		t.Fatalf("namespace changed after confirm %v", nsAgain)
	}
}

func TestHistoryHintDoesNotBindWithoutYes(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)
	srv, hits := historyLinkServer(t, when)

	lg := openHistoryLedger(t, testhome.New(home))
	if err := lg.SetMeta(cursor.HistoryAccountHintMeta, "test-user"); err != nil {
		t.Fatal(err)
	}
	prompt, err := DescribeCursorHistory(lg, "test-user")
	lg.Close()
	if err != nil || !strings.Contains(prompt, "已保存的记录与当前账号一致") || !strings.Contains(prompt, "仍需要你明确确认才会关联") {
		t.Fatalf("prompt=%s err=%v", prompt, err)
	}
	blocked := runHistoryLink(t, home, srv, windowJWT, when)
	if *hits != 0 || !strings.Contains(strings.Join(blocked.Errors, "\n"), cursor.KindUnboundHistory) {
		t.Fatalf("hits=%d errors=%v", *hits, blocked.Errors)
	}
	assertCursorSubject(t, testhome.New(home), "")
}

func TestConflictingHistoryHintRejectsYes(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	home := t.TempDir()
	writeChainCursorDB(t, home, false)
	seedUnboundCursorHistory(t, testhome.New(home))
	when := time.Date(2026, 11, 2, 15, 0, 0, 0, time.UTC)

	lg := openHistoryLedger(t, testhome.New(home))
	if err := lg.SetMeta(cursor.HistoryAccountHintMeta, "other-user"); err != nil {
		t.Fatal(err)
	}
	prompt, err := DescribeCursorHistory(lg, "test-user")
	if err != nil || !strings.Contains(prompt, "已保存的记录指向另一个账号") || strings.Contains(prompt, "other-user") {
		t.Fatalf("prompt=%s err=%v", prompt, err)
	}
	if err := ApplyCursorHistory(lg, "test-user", "yes", when); err == nil || !strings.Contains(err.Error(), cursor.KindHistoryConflict) {
		t.Fatalf("confirm err=%v", err)
	}
	if err := ApplyCursorHistory(lg, "test-user", "Yes", when); err == nil {
		t.Fatal("Yes was accepted")
	}
	lg.Close()
	assertCursorSubject(t, testhome.New(home), "")
	assertNoHistoryDecision(t, testhome.New(home))
	if got := chainDayTotals(t, testhome.New(home), "cursor"); got["2026-10-23"] != chainObserved || got["2025-10-21"] != oct21Total {
		t.Fatalf("history changed %v", got)
	}
}

func seedUnboundCursorHistory(t *testing.T, home adapter.Home) {
	t.Helper()
	lg := openHistoryLedger(t, home)
	defer lg.Close()
	oct21 := event.UsageEvent{
		Source: "cursor", RequestID: "day-2025-10-21",
		Timestamp: time.Date(2025, 10, 21, 12, 0, 0, 0, time.UTC),
		Miss:      539187, CacheRead: 5675130, CacheCreate: 77078, Output: 55585,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveProviderAPI,
		SkipRequest: true,
	}
	if oct21.Miss+oct21.CacheRead+oct21.CacheCreate+oct21.Output != oct21Total {
		t.Fatalf("oct 21 fixture=%d", oct21.Miss+oct21.CacheRead+oct21.CacheCreate+oct21.Output)
	}
	lump := event.UsageEvent{
		Source: "cursor", RequestID: "retained:2026-10-23",
		Timestamp:   time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Miss:        chainObserved,
		Quality:     event.QualityAuthoritative,
		Derivation:  event.DeriveProviderAPI,
		SkipRequest: true,
	}
	err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), []event.UsageEvent{oct21, lump})
	if err != nil {
		t.Fatal(err)
	}
}

func historyLinkServer(t *testing.T, when time.Time) (*httptest.Server, *int) {
	t.Helper()
	var hits int
	nov := time.Date(2026, 11, 1, 0, 0, 0, 0, time.UTC).UnixMilli()
	stamp := when.UnixMilli()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		hits++
		if !strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
			http.NotFound(w, r)
			return
		}
		var body struct {
			Start int64 `json:"startDate"`
		}
		raw, _ := io.ReadAll(r.Body)
		_ = json.Unmarshal(raw, &body)
		if body.Start < nov {
			http.NotFound(w, r)
			return
		}
		_, _ = fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"gpt-5","conversationId":"win","tokenUsage":{"inputTokens":7,"outputTokens":1,"cacheReadTokens":1,"cacheWriteTokens":1}}]}`, stamp)
	}))
	t.Cleanup(srv.Close)
	return srv, &hits
}

func runHistoryLink(t *testing.T, home string, srv *httptest.Server, token string, when time.Time) Result {
	t.Helper()
	return Run(testhome.New(home), []adapter.Adapter{cursor.Adapter{
		HTTP:    srv.Client(),
		APIBase: srv.URL,
		Probe:   false,
		Now:     func() time.Time { return when },
		Unseal:  func(string) (string, error) { return token, nil },
	}})
}

func openHistoryLedger(t *testing.T, home adapter.Home) *ledger.Ledger {
	t.Helper()
	p, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	return lg
}

func assertCursorSubject(t *testing.T, home adapter.Home, want string) {
	t.Helper()
	lg := openHistoryLedger(t, home)
	defer lg.Close()
	got, err := lg.Meta(cursor.AccountSubjectMeta)
	if err != nil || got != want {
		t.Fatalf("subject=%q want=%q err=%v", got, want, err)
	}
}

func assertNoHistoryDecision(t *testing.T, home adapter.Home) {
	t.Helper()
	lg := openHistoryLedger(t, home)
	defer lg.Close()
	for _, key := range []string{cursor.HistoryDecisionMeta, cursor.HistoryAuditSourceMeta, cursor.HistoryBaselineMeta, cursor.HistoryDecisionSubjectMeta} {
		got, err := lg.Meta(key)
		if err != nil || got != "" {
			t.Fatalf("%s=%q err=%v", key, got, err)
		}
	}
}

func historyAudit(t *testing.T, lg *ledger.Ledger) (baseline, at string) {
	t.Helper()
	var err error
	baseline, err = lg.Meta(cursor.HistoryBaselineMeta)
	if err != nil || baseline == "" {
		t.Fatalf("baseline=%q err=%v", baseline, err)
	}
	at, err = lg.Meta(cursor.HistoryAuditAtMeta)
	if err != nil || at == "" {
		t.Fatalf("audit at=%q err=%v", at, err)
	}
	return baseline, at
}

func chainDayTotals(t *testing.T, home adapter.Home, source string) map[string]int64 {
	t.Helper()
	lg := openHistoryLedger(t, home)
	defer lg.Close()
	chained, ok, err := lg.ChainedRetainedEvents(source)
	if err != nil || !ok {
		t.Fatalf("source=%s ok=%v err=%v", source, ok, err)
	}
	got := map[string]int64{}
	for _, e := range chained {
		got[e.Timestamp.UTC().Format("2006-01-02")] += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	return got
}

func assertHistoryLinkErrors(t *testing.T, errs []string) {
	t.Helper()
	joined := strings.Join(errs, "\n")
	low := strings.ToLower(joined)
	if strings.Contains(low, "auth") || strings.Contains(joined, "eyJ") || strings.Contains(joined, "test-user") || strings.Contains(joined, "other-user") {
		t.Fatalf("errors leaked material: %v", errs)
	}
	if strings.Contains(joined, cursor.KindUnboundHistory) || strings.Contains(joined, cursor.KindAccountMismatch) {
		t.Fatalf("errors=%v", errs)
	}
}
