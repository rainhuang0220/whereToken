package scan

import (
	"fmt"
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
