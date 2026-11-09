package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

// The testApp fixture prices like this:
//   - claude-opus-4.6 (anthropic): 1.00 M miss × $5.00, 9.00 M cache read ×
//     $0.50, 0.10 M output × $25.00 → $12.0000, complete.
//   - MiniMax-M3 (minimax) and k3 (moonshot): no public card row →
//     raw model labels, 1.58 M unpriced, never $0.
//   - Total: 11.68 M, $12.0000, partial.

func TestPricingUsageHumanBreakdown(t *testing.T) {
	code, out, errb := runPricing(t, "pricing", "--usage")
	if code != ExitOK {
		t.Fatalf("exit %d stderr=%s", code, errb)
	}
	for _, want := range []string{
		"单价单位：美元 / 每百万 tokens（USD per 1M tokens）",
		"周期：有账本以来",
		"Anthropic（anthropic）",
		"claude-opus-4.6",
		"1.00 M × $5.00",
		"9.00 M × $0.50",
		"0.10 M × $25.00",
		"$12.0000",
		"MiniMax（minimax）",
		"Moonshot（moonshot）",
		"MiniMax-M3",
		"k3",
		"0.50 M × —",
		"0.80 M × —",
		"部分用量无公开价",
		"总计 11.68 M · 估价 $12.0000 · 部分用量无公开价",
		"不是订阅账单",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("pricing --usage missing %q:\n%s", want, out)
		}
	}
	if strings.Contains(out, "$0.00") {
		t.Fatalf("no fake zeros in usage pricing:\n%s", out)
	}
	// The price card itself must not leak into the usage view.
	if strings.Contains(out, "价目卡") {
		t.Fatalf("usage mode must not print the catalog header:\n%s", out)
	}
}

func TestPricingUsageWindowFlags(t *testing.T) {
	for _, args := range [][]string{
		{"pricing", "--usage", "--today"},
		{"pricing", "--usage", "--since", "1d"},
	} {
		code, out, errb := runPricing(t, args...)
		if code != ExitOK {
			t.Fatalf("%v: exit %d stderr=%s", args, code, errb)
		}
		// Fixture now is 2026-08-16; the priced anthropic event is 2026-08-15.
		if strings.Contains(out, "claude-opus-4.6") {
			t.Fatalf("%v leaked yesterday's rows:\n%s", args, out)
		}
		if !strings.Contains(out, "总计 1.58 M · 估价 —") {
			t.Fatalf("%v: today is 1.58 M, all unpriced:\n%s", args, out)
		}
	}
	code, out, errb := runPricing(t, "pricing", "--usage", "--from", "2026-08-15", "--to", "2026-08-15")
	if code != ExitOK {
		t.Fatalf("--from/--to: exit %d stderr=%s", code, errb)
	}
	if !strings.Contains(out, "总计 10.10 M · 估价 $12.0000") {
		t.Fatalf("2026-08-15 alone is the priced anthropic day:\n%s", out)
	}
	if strings.Contains(out, "部分用量无公开价") {
		t.Fatalf("that day prices completely:\n%s", out)
	}
}

func TestPricingUsageVendorAndModelFilters(t *testing.T) {
	code, out, errb := runPricing(t, "pricing", "--usage", "--vendor", "anthropic")
	if code != ExitOK {
		t.Fatalf("exit %d stderr=%s", code, errb)
	}
	if !strings.Contains(out, "Anthropic（anthropic）") || !strings.Contains(out, "总计 10.10 M · 估价 $12.0000") {
		t.Fatalf("--vendor anthropic:\n%s", out)
	}
	for _, gone := range []string{"MiniMax（minimax）", "Moonshot（moonshot）", "部分用量无公开价"} {
		if strings.Contains(out, gone) {
			t.Fatalf("--vendor anthropic must hide %q:\n%s", gone, out)
		}
	}

	_, out, _ = runPricing(t, "pricing", "--usage", "--model", "opus")
	if !strings.Contains(out, "claude-opus-4.6") || strings.Contains(out, "未知模型") {
		t.Fatalf("--model opus keeps only the opus row:\n%s", out)
	}
	if !strings.Contains(out, "总计 10.10 M · 估价 $12.0000") {
		t.Fatalf("--model opus total:\n%s", out)
	}
}

func TestPricingUsageJSONExtendsCatalog(t *testing.T) {
	code, out, errb := runPricing(t, "pricing", "--usage", "--json")
	if code != ExitOK {
		t.Fatalf("exit %d stderr=%s", code, errb)
	}
	var payload struct {
		Card  string `json:"card"`
		Unit  string `json:"unit"`
		Usage struct {
			Period  string `json:"period"`
			Vendors []struct {
				Vendor string `json:"vendor"`
				Label  string `json:"label"`
				Models []struct {
					Label      string `json:"label"`
					Total      int64  `json:"total"`
					CostStatus string `json:"cost_status"`
					CostUSD    string `json:"cost_usd"`
					UnitPrices struct {
						Miss        *float64 `json:"miss"`
						CacheRead   *float64 `json:"cache_read"`
						CacheCreate *float64 `json:"cache_create"`
						Output      *float64 `json:"output"`
					} `json:"unit_prices"`
					UnpricedTokens int64 `json:"unpriced_tokens"`
				} `json:"models"`
			} `json:"vendors"`
			Total struct {
				Total          int64  `json:"total"`
				TotalM         string `json:"total_m"`
				CostStatus     string `json:"cost_status"`
				CostUSD        string `json:"cost_usd"`
				UnpricedTokens int64  `json:"unpriced_tokens"`
			} `json:"total"`
		} `json:"usage"`
	}
	if err := json.Unmarshal([]byte(out), &payload); err != nil {
		t.Fatalf("%s", out)
	}
	if payload.Card != "2026-08-19" || payload.Unit != "usd_per_1m_tokens" {
		t.Fatalf("catalog keys must stay: %+v", payload)
	}
	if payload.Usage.Period != "有账本以来" {
		t.Fatalf("period: %+v", payload.Usage)
	}
	if payload.Usage.Total.Total != 11_680_000 || payload.Usage.Total.TotalM != "11.68 M" {
		t.Fatalf("total: %+v", payload.Usage.Total)
	}
	if payload.Usage.Total.CostStatus != "partial" || payload.Usage.Total.CostUSD != "$12.0000" {
		t.Fatalf("total cost: %+v", payload.Usage.Total)
	}
	if payload.Usage.Total.UnpricedTokens != 1_580_000 {
		t.Fatalf("unpriced: %+v", payload.Usage.Total)
	}
	foundPriced, foundUnknown := false, false
	for _, v := range payload.Usage.Vendors {
		for _, m := range v.Models {
			if v.Vendor == "anthropic" {
				foundPriced = true
				if m.Label != "claude-opus-4.6" || m.CostStatus != "complete" || m.CostUSD != "$12.0000" {
					t.Fatalf("anthropic row: %+v", m)
				}
				if m.UnitPrices.Miss == nil || *m.UnitPrices.Miss != 5 {
					t.Fatalf("miss rate must be the listed $5: %+v", m.UnitPrices)
				}
				if m.UnitPrices.CacheRead == nil || *m.UnitPrices.CacheRead != 0.5 {
					t.Fatalf("cache read rate $0.50: %+v", m.UnitPrices)
				}
			}
			if v.Vendor == "minimax" {
				foundUnknown = true
				if m.Label != "MiniMax-M3" || m.CostStatus != "unavailable" || m.CostUSD != "" {
					t.Fatalf("unpriced row stays unavailable, never $0: %+v", m)
				}
				if m.UnitPrices.Miss != nil || m.UnitPrices.Output != nil {
					t.Fatalf("no card → no rates: %+v", m.UnitPrices)
				}
				if m.UnpricedTokens != 550_000 {
					t.Fatalf("unpriced tokens: %+v", m)
				}
			}
		}
	}
	if !foundPriced || !foundUnknown {
		t.Fatalf("vendors: %+v", payload.Usage.Vendors)
	}
	// The catalog payload is unchanged: eight providers ride along.
	var raw struct {
		Providers []map[string]any `json:"providers"`
	}
	if err := json.Unmarshal([]byte(out), &raw); err != nil {
		t.Fatal(err)
	}
	if len(raw.Providers) != 8 {
		t.Fatalf("providers: %d", len(raw.Providers))
	}
}

func TestPricingJSONWithoutUsageHasNoUsageKey(t *testing.T) {
	_, out, _ := runPricing(t, "pricing", "--json")
	var m map[string]any
	if err := json.Unmarshal([]byte(out), &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["usage"]; ok {
		t.Fatalf("plain pricing --json must not grow a usage block:\n%s", out)
	}
}

func TestPricingUsageStillRejectsToolAndNoCommunity(t *testing.T) {
	for _, args := range [][]string{
		{"pricing", "--usage", "--tool", "claude"},
		{"pricing", "--usage", "--no-community"},
	} {
		code, _, errb := runPricing(t, args...)
		if code != ExitUsage || !strings.Contains(errb, "pricing") {
			t.Fatalf("%v: code=%d stderr=%q", args, code, errb)
		}
	}
	// --offline stays legal in usage mode: the scan simply skips cloud APIs.
	code, _, errb := runPricing(t, "pricing", "--usage", "--offline")
	if code != ExitOK {
		t.Fatalf("pricing --usage --offline: code=%d stderr=%q", code, errb)
	}
}

func TestPricingUsageEmptyHomeIsHonest(t *testing.T) {
	app, out, errb := testApp([]string{"pricing", "--usage"})
	app.Scan = func(adapter.Home) scan.Result { return scan.Result{} }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if !strings.Contains(s, "没有可估价的用量") {
		t.Fatalf("empty home must say so:\n%s", s)
	}
	if strings.Contains(s, "$0.00") {
		t.Fatalf("empty is not $0:\n%s", s)
	}
}

func TestPricingUsageOnRealFixtureHome(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dst := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	app, out, errb := testApp([]string{"pricing", "--usage", "--home", dir})
	app.Scan = nil
	app.Home = nil
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	// kimi-code/k3 has no public card row: real tokens, unavailable price.
	for _, want := range []string{
		"单价单位：美元 / 每百万 tokens",
		"Moonshot（moonshot）",
		"kimi-code/k3",
		"总计 0.0012 M · 估价 — · 部分用量无公开价",
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("fixture home missing %q:\n%s", want, s)
		}
	}
	if strings.Contains(s, "$0.00") {
		t.Fatalf("unpriced is never $0:\n%s", s)
	}
}

func TestHelpDocumentsPricingUsageAndPublicSite(t *testing.T) {
	h := HelpText()
	if !strings.Contains(h, "pricing --usage") {
		t.Fatalf("help must document pricing --usage:\n%s", h)
	}
	if !strings.Contains(h, "https://rainhuang0220.github.io/whereToken/") {
		t.Fatalf("help must point at the public demo site:\n%s", h)
	}
}

func TestCompletionPricingOffersUsage(t *testing.T) {
	for _, sh := range []string{"bash", "zsh", "fish", "powershell"} {
		s, err := Completion(sh)
		if err != nil {
			t.Fatal(err)
		}
		switch sh {
		case "bash":
			if !strings.Contains(s, `pricing) opts="--vendor --model --json --usage `) {
				t.Fatalf("bash pricing must offer --usage:\n%s", s)
			}
		case "zsh":
			i, j := strings.Index(s, "    pricing)"), strings.Index(s, "    *)")
			if i < 0 || j <= i || !strings.Contains(s[i:j], "'--usage[") {
				t.Fatalf("zsh pricing must offer --usage:\n%s", s[i:j])
			}
		case "fish":
			if !strings.Contains(s, `__fish_seen_subcommand_from pricing" -l usage`) {
				t.Fatalf("fish pricing must offer --usage:\n%s", s)
			}
		case "powershell":
			if !strings.Contains(s, `'--usage'`) {
				t.Fatalf("powershell pricing must offer --usage:\n%s", s)
			}
		}
	}
}

func TestPricingUsageRetainsUnpricedModelAndExactPricedAlias(t *testing.T) {
	for _, tc := range []struct {
		query, wantID string
		total         int64
	}{
		{"GROK-BOT-AUTOMATION", "grok-bot-automation", 111},
		{"cursor-grok-4.6-xhigh-fast", "cursor-grok-4.6-xhigh-fast", 222},
		{"claude-4.6-opus-high-thinking", "claude-opus-4.6", 100},
	} {
		t.Run(tc.query, func(t *testing.T) {
			app, out, errb := testApp([]string{"pricing", "--usage", "--model", tc.query, "--json"})
			app.Scan = func(adapter.Home) scan.Result {
				return scan.Result{Events: []event.UsageEvent{
					{Source: "cursor", Vendor: "xai", Model: "grok-bot-automation", RequestID: "a", Miss: 111},
					{Source: "cursor", Vendor: "xai", Model: "cursor-grok-4.6-xhigh-fast", RequestID: "b", Miss: 222},
					{Source: "cursor", Vendor: "anthropic", Model: "claude-4.6-opus-high-thinking", RequestID: "c", Timestamp: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), Miss: 100},
					{Source: "cursor", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "d", Timestamp: time.Date(2026, 8, 15, 0, 0, 0, 0, time.UTC), Miss: 200},
				}}
			}
			if code := app.Run(); code != ExitOK {
				t.Fatalf("exit=%d %s", code, errb.String())
			}
			var got pricingJSON
			if e := json.Unmarshal(out.Bytes(), &got); e != nil {
				t.Fatal(e)
			}
			if got.Usage.Total.Total != tc.total || len(got.Usage.Vendors) != 1 || len(got.Usage.Vendors[0].Models) != 1 {
				t.Fatalf("wrong exact model scope: %s", out.String())
			}
			m := got.Usage.Vendors[0].Models[0]
			if m.ID != tc.wantID {
				t.Fatalf("model id=%q want=%q", m.ID, tc.wantID)
			}
			switch m.Vendor {
			case "xai":
				if m.CostStatus != "unavailable" || m.CostUSD != "" || m.UnpricedTokens != tc.total {
					t.Fatalf("unpriced model acquired cost: %+v", m)
				}
			case "anthropic":
				if m.CostStatus != "complete" || m.CostUSD == "" || m.UnpricedTokens != 0 || m.Total != tc.total {
					t.Fatalf("priced alias lost its own total or gained a fake zero: %+v", m)
				}
			default:
				t.Fatalf("vendor %q", m.Vendor)
			}
		})
	}
}

func TestPricingUsageFamilyQueryDoesNotAbsorbUnpricedEffortVariant(t *testing.T) {
	when := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	app, out, errb := testApp([]string{"pricing", "--usage", "--model=grok-4.6", "--json"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Events: []event.UsageEvent{
			{Source: "cursor", Vendor: "xai", Model: "cursor-grok-4.6-xhigh-fast", RequestID: "effort", Timestamp: when, Miss: 50},
		}}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d %s", code, errb.String())
	}
	var got pricingJSON
	if e := json.Unmarshal(out.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.Usage.Total.Total != 0 || len(got.Usage.Vendors) != 0 {
		t.Fatalf("family query absorbed an unpriced effort variant: %s", out.String())
	}
}

func TestPricingUsageShortQueryKeepsPricedCardOnly(t *testing.T) {
	when := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	app, out, errb := testApp([]string{"pricing", "--usage", "--model=k3", "--json"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Events: []event.UsageEvent{
			{Source: "cursor", Vendor: "moonshot", Model: "kimi-k3", RequestID: "card", Timestamp: when, Miss: 1_000_000},
			{Source: "cursor", Vendor: "moonshot", Model: "kimi-code/k3", RequestID: "path", Timestamp: when, Miss: 7},
		}}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d %s", code, errb.String())
	}
	var got pricingJSON
	if e := json.Unmarshal(out.Bytes(), &got); e != nil {
		t.Fatal(e)
	}
	if got.Usage.Total.Total != 1_000_000 || len(got.Usage.Vendors) != 1 || len(got.Usage.Vendors[0].Models) != 1 {
		t.Fatalf("short query mixed a path model into the priced card: %s", out.String())
	}
	if got.Usage.Vendors[0].Models[0].ID != "kimi-k3" {
		t.Fatalf("model=%s", got.Usage.Vendors[0].Models[0].ID)
	}
}

func TestPricingUsagePartialRowDoesNotApplyRateToUnpricedTokens(t *testing.T) {
	app, out, errb := testApp([]string{"pricing", "--usage", "--model=gpt-5"})
	app.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Events: []event.UsageEvent{
			{Source: "cursor", Vendor: "openai", Model: "gpt-5", RequestID: "before", Timestamp: time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC), Miss: 100},
			{Source: "cursor", Vendor: "openai", Model: "gpt-5", RequestID: "after", Timestamp: time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC), Miss: 100},
		}}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d %s", code, errb.String())
	}
	text := out.String()
	if strings.Contains(text, "$1.25") || !strings.Contains(text, "× —") || !strings.Contains(text, "部分用量无公开价") {
		t.Fatalf("partial row priced unpriced tokens: %s", text)
	}
}

func TestPricingUsageHumanExactModelMatchesJSONScope(t *testing.T) {
	when := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "xai", Model: "grok-4.6", RequestID: "priced", Timestamp: when, Miss: 1_000_000},
		{Source: "cursor", Vendor: "xai", Model: "cursor-grok-4.6-xhigh-fast", RequestID: "effort", Timestamp: when, Miss: 50},
	}
	scan := func(adapter.Home) scan.Result { return scan.Result{Events: events} }
	app, out, errb := testApp([]string{"pricing", "--usage", "--model=grok-4.6"})
	app.Scan = scan
	if code := app.Run(); code != ExitOK {
		t.Fatalf("exit=%d %s", code, errb.String())
	}
	text := out.String()
	if !strings.Contains(text, "grok-4.6") || strings.Contains(text, "cursor-grok-4.6-xhigh-fast") || strings.Contains(text, "未知模型") {
		t.Fatalf("human scope drifted: %s", text)
	}
	if !strings.Contains(text, "$2.0000") {
		t.Fatalf("human price drifted: %s", text)
	}
}

func TestPricingUsageExactScopeDoesNotFoldSiblingOrOtherVendor(t *testing.T) {
	when := time.Date(2026, 8, 16, 0, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "xai", Model: "grok-4.6", RequestID: "priced", Timestamp: when, Miss: 1_000_000},
		{Source: "cursor", Vendor: "xai", Model: "cursor-grok-4.6-xhigh-fast", RequestID: "effort", Timestamp: when, Miss: 50},
		{Source: "cursor", Vendor: "moonshot", Model: "kimi-code/k3", RequestID: "slash", Timestamp: when, Miss: 7},
		{Source: "cursor", Vendor: "moonshot", Model: "k3", RequestID: "raw-k3", Timestamp: when, Miss: 9},
		{Source: "cursor", Vendor: "xai", Model: "k3", RequestID: "other-vendor", Timestamp: when, Miss: 11},
	}
	for _, tc := range []struct {
		name, vendor, model string
		total               int64
		ids                 []string
		unpriced            bool
	}{
		{name: "priced card excludes effort variant", model: "grok-4.6", total: 1_000_000, ids: []string{"grok-4.6"}},
		{name: "effort variant stays alone", model: "cursor-grok-4.6-xhigh-fast", total: 50, ids: []string{"cursor-grok-4.6-xhigh-fast"}, unpriced: true},
		{name: "full path beats short name", model: "kimi-code/k3", total: 7, ids: []string{"kimi-code/k3"}, unpriced: true},
		{name: "raw k3 does not take the path model", model: "k3", total: 20, ids: []string{"k3", "k3"}, unpriced: true},
		{name: "vendor narrows same model name", vendor: "xai", model: "k3", total: 11, ids: []string{"k3"}, unpriced: true},
		{name: "family fallback still folds the priced alias", model: "opus-4.6", total: 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			args := []string{"pricing", "--usage", "--json"}
			if tc.vendor != "" {
				args = append(args, "--vendor="+tc.vendor)
			}
			if tc.model != "" {
				args = append(args, "--model="+tc.model)
			}
			app, out, errb := testApp(args)
			app.Scan = func(adapter.Home) scan.Result {
				ev := append([]event.UsageEvent{}, events...)
				if tc.model == "opus-4.6" {
					ev = append(ev,
						event.UsageEvent{Source: "cursor", Vendor: "anthropic", Model: "claude-4.6-opus-high-thinking", RequestID: "hi", Timestamp: when, Miss: 100},
						event.UsageEvent{Source: "cursor", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "plain", Timestamp: when, Miss: 200},
					)
				}
				return scan.Result{Events: ev}
			}
			if code := app.Run(); code != ExitOK {
				t.Fatalf("exit=%d %s", code, errb.String())
			}
			var got pricingJSON
			if e := json.Unmarshal(out.Bytes(), &got); e != nil {
				t.Fatal(e)
			}
			wantTotal := tc.total
			wantIDs := tc.ids
			if tc.model == "opus-4.6" {
				wantTotal = 300
				wantIDs = []string{"claude-opus-4.6"}
			}
			if got.Usage.Total.Total != wantTotal {
				t.Fatalf("total=%d want=%d body=%s", got.Usage.Total.Total, wantTotal, out.String())
			}
			var ids []string
			var gotTokens int64
			for _, v := range got.Usage.Vendors {
				if tc.vendor != "" && v.Vendor != tc.vendor {
					t.Fatalf("vendor leaked: %+v", v)
				}
				for _, m := range v.Models {
					ids = append(ids, m.ID)
					gotTokens += m.Total
					if tc.unpriced && (m.CostStatus != "unavailable" || m.CostUSD != "" || m.UnpricedTokens != m.Total) {
						t.Fatalf("unpriced row acquired a price: %+v", m)
					}
				}
			}
			if gotTokens != wantTotal {
				t.Fatalf("model tokens %d != total %d", gotTokens, wantTotal)
			}
			if strings.Join(ids, ",") != strings.Join(wantIDs, ",") {
				t.Fatalf("ids=%v want=%v body=%s", ids, wantIDs, out.String())
			}
		})
	}
}
