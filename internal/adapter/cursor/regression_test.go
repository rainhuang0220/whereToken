package cursor

import (
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

// Guard for the 2026-08 production incident: a Cursor ledger with ~3B API
// tokens estimated at "$6 · 部分". Cursor's Dashboard API reports model ids
// version-first (claude-4.6-opus-high-thinking, claude-4.5-sonnet, gpt-5-high)
// while the price card patterns are family-first (opus-4.6, sonnet-4.5, gpt-5),
// so every Claude token fell out of pricing and only a small bare-gpt-5 slice
// was charged. The fix lives in price.Normalize/Resolve; this test pins the
// whole path end to end: API page -> adapter events -> metric.Aggregate ->
// exact per-model cost from the card (USD per 1M: opus-4.6 5/0.5/6.25/25,
// sonnet-4.5 3/0.3/3.75/15, gpt-5 1.25/0.125/unlisted/10). Synthetic fixture,
// no real user data. If the card rates change intentionally, update the
// expectations here.
func TestVersionFirstModelIDsPriceAgainstCard(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case strings.Contains(r.URL.Path, "GetFilteredUsageEvents"):
			io.WriteString(w, `{
			  "usageEventsDisplay": [
			    {
			      "timestamp": "1770000000000",
			      "model": "claude-4.6-opus-high-thinking",
			      "conversationId": "sess-a",
			      "tokenUsage": {
			        "inputTokens": "2000000000",
			        "cacheReadTokens": "500000000",
			        "cacheWriteTokens": "100000000",
			        "outputTokens": "50000000"
			      }
			    },
			    {
			      "timestamp": "1770086400000",
			      "model": "claude-4.5-sonnet",
			      "conversationId": "sess-b",
			      "tokenUsage": {
			        "inputTokens": "300000000",
			        "cacheReadTokens": "60000000",
			        "cacheWriteTokens": "10000000",
			        "outputTokens": "20000000"
			      }
			    },
			    {
			      "timestamp": "1770172800000",
			      "model": "gpt-5-high",
			      "conversationId": "sess-c",
			      "tokenUsage": {
			        "inputTokens": "400000000",
			        "cacheReadTokens": "80000000",
			        "cacheWriteTokens": "0",
			        "outputTokens": "30000000"
			      }
			    }
			  ],
			  "totalUsageEventsCount": 3
			}`)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(srv.Close)

	dir := t.TempDir()
	// Local bubbles carry small tokenCount values; the API has totals, so they
	// must be stripped (existing behavior) rather than double counted.
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`},
	}, nil)
	putItem(t, db, authAccessTokenKey, fakeJWT)

	var evs []event.UsageEvent
	a := Adapter{HTTP: srv.Client(), APIBase: srv.URL}
	if err := a.Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}

	// Raw ids stay verbatim on events (display); normalization is pricing's job.
	var sawRaw map[string]bool = map[string]bool{}
	for _, e := range evs {
		sawRaw[e.Model] = true
	}
	for _, raw := range []string{"claude-4.6-opus-high-thinking", "claude-4.5-sonnet", "gpt-5-high"} {
		if !sawRaw[raw] {
			t.Fatalf("raw model id %q lost from events: %v", raw, sawRaw)
		}
	}

	sum := metric.Aggregate(evs, nil)

	// 3.6B tokens exactly as the API reported; the local 100/10 is stripped.
	if sum.All.Miss != 2_700_000_000 || sum.All.CacheRead != 640_000_000 ||
		sum.All.CacheCreate != 110_000_000 || sum.All.Output != 100_000_000 {
		t.Fatalf("tokens %+v (API is authoritative, no local double count)", sum.All)
	}

	// The incident number was ~$6 on ~3B tokens. Exact card math:
	//   opus-4.6:   2e9*5 + 5e8*0.5 + 1e8*6.25 + 5e7*25      = $12,125.00
	//   sonnet-4.5: 3e8*3 + 6e7*0.3 + 1e7*3.75 + 2e7*15      = $1,255.50
	//   gpt-5:      4e8*1.25 + 8e7*0.125 + 3e7*10            = $810.00
	const wantMicro = 14_190_500_000 // $14,190.50
	if sum.All.CostMicro != wantMicro {
		t.Fatalf("cost=%d micro ($%.2f), want %d ($14,190.50) — version-first ids must price against the card",
			sum.All.CostMicro, float64(sum.All.CostMicro)/1e6, wantMicro)
	}
	if sum.All.CostStatus != price.StatusComplete {
		t.Fatalf("cost status=%q want complete (3B tokens must not collapse to partial)", sum.All.CostStatus)
	}

	byModel := map[string]metric.ModelSlice{}
	for _, m := range sum.ByModel {
		byModel[m.Vendor+"/"+m.Model] = m
	}
	if len(sum.ByModel) != 3 {
		t.Fatalf("ByModel rows=%d want 3: %+v", len(sum.ByModel), sum.ByModel)
	}

	type wantModel struct {
		costMicro           int64
		miss, cr, cc, out   float64
		cacheCreateUnlisted bool
	}
	wants := map[string]wantModel{
		"anthropic/claude-opus-4.6":   {costMicro: 12_125_000_000, miss: 5, cr: 0.5, cc: 6.25, out: 25},
		"anthropic/claude-sonnet-4.5": {costMicro: 1_255_500_000, miss: 3, cr: 0.3, cc: 3.75, out: 15},
		"openai/gpt-5":                {costMicro: 810_000_000, miss: 1.25, cr: 0.125, out: 10, cacheCreateUnlisted: true},
	}
	var modelCostSum int64
	for key, w := range wants {
		m, ok := byModel[key]
		if !ok {
			t.Fatalf("ByModel missing %s", key)
		}
		if m.CostMicro != w.costMicro {
			t.Fatalf("%s cost=%d want %d", key, m.CostMicro, w.costMicro)
		}
		if m.Rate == nil {
			t.Fatalf("%s has no rate card attached", key)
		}
		if m.Rate.Miss != w.miss || m.Rate.CacheRead != w.cr || m.Rate.CacheCreate != w.cc || m.Rate.Output != w.out {
			t.Fatalf("%s rate %+v want %v/%v/%v/%v", key, *m.Rate, w.miss, w.cr, w.cc, w.out)
		}
		v := metric.ViewModel(m)
		if v.UnitPrices.Miss == nil || *v.UnitPrices.Miss != w.miss ||
			v.UnitPrices.CacheRead == nil || *v.UnitPrices.CacheRead != w.cr ||
			v.UnitPrices.Output == nil || *v.UnitPrices.Output != w.out {
			t.Fatalf("%s unit prices %+v incomplete", key, v.UnitPrices)
		}
		if w.cacheCreateUnlisted {
			if v.UnitPrices.CacheCreate != nil {
				t.Fatalf("%s cache-create must stay unlisted (nil), not free: %+v", key, v.UnitPrices)
			}
		} else if v.UnitPrices.CacheCreate == nil || *v.UnitPrices.CacheCreate != w.cc {
			t.Fatalf("%s cache-create unit price %+v want %v", key, v.UnitPrices.CacheCreate, w.cc)
		}
		modelCostSum += m.CostMicro
	}
	if modelCostSum != sum.All.CostMicro {
		t.Fatalf("ByModel cost sum %d != All %d", modelCostSum, sum.All.CostMicro)
	}
}
