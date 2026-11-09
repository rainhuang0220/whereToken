package scan

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/cursor"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestCursorAttributionBackfillGuardsAccountAndConservesBaseline(t *testing.T) {
	for _, tc := range []struct {
		name, identity string
		wantCalls      int
	}{
		{"same-account", windowJWT, 2}, {"different-account", otherJWT, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dir := t.TempDir()
			home := testhome.New(dir)
			writeChainCursorDB(t, dir, false)
			if err := cursor.WriteConsent(cursor.ConsentPath(home)); err != nil {
				t.Fatal(err)
			}
			lg := openHistoryLedger(t, home)
			stamp := time.Date(2026, 11, 2, 12, 0, 0, 0, time.UTC)
			if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", stamp, []event.UsageEvent{{Source: "cursor", Timestamp: stamp, Miss: 10, CacheRead: 20, CacheCreate: 5, Output: 3}}); err != nil {
				t.Fatal(err)
			}
			if err := lg.SetMeta(cursor.AccountSubjectMeta, "test-user"); err != nil {
				t.Fatal(err)
			}
			lg.Close()
			calls := 0
			srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls++
				if !strings.Contains(r.URL.Path, "GetFilteredUsageEvents") {
					t.Error("unexpected endpoint")
					http.NotFound(w, r)
					return
				}
				fmt.Fprintf(w, `{"totalUsageEventsCount":1,"usageEventsDisplay":[{"timestamp":"%d","model":"grok-4.7","tokenUsage":{"inputTokens":3,"cacheReadTokens":4,"cacheWriteTokens":1,"outputTokens":2}}]}`, stamp.UnixMilli())
			}))
			defer srv.Close()
			ad := cursor.Adapter{HTTP: srv.Client(), APIBase: srv.URL, Unseal: func(string) (string, error) { return tc.identity, nil }, Now: func() time.Time { return stamp.AddDate(0, 0, 2) }}
			for i := 0; i < 2; i++ {
				got, err := BackfillCursorAttribution(home, ad, "2026-11-02", "2026-11-02")
				if tc.wantCalls > 0 && (err != nil || got.Before != 38 || got.After != 38 || got.AllocatedDays != 1) {
					t.Fatalf("result=%+v err=%v", got, err)
				}
				if tc.wantCalls == 0 && (err == nil || !strings.Contains(err.Error(), cursor.KindAccountMismatch)) {
					t.Fatalf("different account not blocked: %v", err)
				}
			}
			if calls != tc.wantCalls {
				t.Fatalf("calls=%d want=%d", calls, tc.wantCalls)
			}
			lg = openHistoryLedger(t, home)
			defer lg.Close()
			events, _, err := lg.ChainedRetainedEvents("cursor")
			if err != nil {
				t.Fatal(err)
			}
			sum := metric.Aggregate(events, nil)
			if sum.All.Total() != 38 {
				t.Fatalf("total changed: %d", sum.All.Total())
			}
			var xai int64
			for _, e := range events {
				if e.Vendor == "xai" {
					xai += e.Miss + e.CacheRead + e.CacheCreate + e.Output
				}
			}
			want := int64(0)
			if tc.wantCalls > 0 {
				want = 10
			}
			if xai != want {
				t.Fatalf("xai=%d want=%d", xai, want)
			}
		})
	}
}
