package metric

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func modelRow(rows []ModelSlice, vendor, model string) *ModelSlice {
	for i := range rows {
		if rows[i].Vendor == vendor && rows[i].Model == model {
			return &rows[i]
		}
	}
	return nil
}

func TestByModelTwoModelsSameVendor(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a",
			Miss: 1_000_000, CacheRead: 2_000_000, CacheCreate: 100_000, Output: 50_000},
		{Source: "claude", Vendor: "anthropic", Model: "claude-sonnet-4.5", RequestID: "b",
			Miss: 500_000, CacheRead: 1_000_000, Output: 100_000},
	}
	sum := Aggregate(events, nil)
	if len(sum.ByModel) != 2 {
		t.Fatalf("by_model rows=%d %+v", len(sum.ByModel), sum.ByModel)
	}
	// Sorted by Total desc: opus 3.15M > sonnet 1.6M.
	if sum.ByModel[0].Model != "claude-opus-4.6" || sum.ByModel[1].Model != "claude-sonnet-4.5" {
		t.Fatalf("order %+v", sum.ByModel)
	}
	opus := &sum.ByModel[0]
	if opus.ID != "claude-opus-4.6" || opus.Label != "claude-opus-4.6" || opus.Vendor != "anthropic" {
		t.Fatalf("opus identity %+v", opus)
	}
	if opus.Rate == nil || opus.Rate.Model != "opus-4.6" {
		t.Fatalf("opus rate %+v", opus.Rate)
	}
	// opus-4.6 card: 5 / 0.50 / 6.25 / 25 USD per 1M.
	if opus.MissCostMicro != 5_000_000 || opus.CacheReadCostMicro != 1_000_000 ||
		opus.CacheCreateCostMicro != 625_000 || opus.OutputCostMicro != 1_250_000 {
		t.Fatalf("opus costs %+v", opus)
	}
	if opus.CostMicro != 7_875_000 || opus.CostStatus != "complete" {
		t.Fatalf("opus total %+v", opus)
	}
	sonnet := &sum.ByModel[1]
	if sonnet.Rate == nil || sonnet.Rate.Model != "sonnet-4.5" {
		t.Fatalf("sonnet rate %+v", sonnet.Rate)
	}
	// sonnet-4.5 card: 3 / 0.30 / 3.75 / 15 USD per 1M.
	if sonnet.MissCostMicro != 1_500_000 || sonnet.CacheReadCostMicro != 300_000 ||
		sonnet.CacheCreateCostMicro != 0 || sonnet.OutputCostMicro != 1_500_000 {
		t.Fatalf("sonnet costs %+v", sonnet)
	}
	if sonnet.CostMicro != 3_300_000 {
		t.Fatalf("sonnet total %+v", sonnet)
	}
	if got := opus.CostMicro + sonnet.CostMicro; got != sum.All.CostMicro {
		t.Fatalf("by_model cost %d != all %d", got, sum.All.CostMicro)
	}
}

func TestByModelGroupsVersionFirstIDs(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "anthropic", Model: "claude-4.6-opus-high-thinking", RequestID: "a", Miss: 100, Output: 10},
		{Source: "cursor", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "b", Miss: 200, Output: 20},
	}
	sum := Aggregate(events, nil)
	if len(sum.ByModel) != 1 {
		t.Fatalf("rows=%d %+v", len(sum.ByModel), sum.ByModel)
	}
	row := &sum.ByModel[0]
	if row.Model != "claude-opus-4.6" || row.Label != "claude-opus-4.6" || row.Vendor != "anthropic" {
		t.Fatalf("row %+v", row)
	}
	if row.Total() != 330 {
		t.Fatalf("total=%d", row.Total())
	}
	if row.Rate == nil || row.Rate.Model != "opus-4.6" {
		t.Fatalf("rate %+v", row.Rate)
	}
}

func TestByModelUnknownModelBucket(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "cursor", Model: "auto", RequestID: "a", Miss: 400_000, Output: 40_000},
		{Source: "claude", Vendor: "anthropic", Model: "claude-sonnet-4.5", RequestID: "b", Miss: 1_000_000},
	}
	sum := Aggregate(events, nil)
	unknown := modelRow(sum.ByModel, "cursor", "")
	if unknown == nil {
		t.Fatalf("no unknown bucket %+v", sum.ByModel)
	}
	if unknown.Label != "(未知模型)" || unknown.Rate != nil {
		t.Fatalf("unknown bucket %+v", unknown)
	}
	if unknown.CostStatus != "unavailable" || unknown.UnpricedTokens != 440_000 || unknown.CostMicro != 0 {
		t.Fatalf("unknown cost %+v", unknown)
	}
	var cost int64
	for _, m := range sum.ByModel {
		cost += m.CostMicro
	}
	if cost != sum.All.CostMicro {
		t.Fatalf("by_model cost %d != all %d", cost, sum.All.CostMicro)
	}
	v := ViewModel(*unknown)
	if v.CostUSD != "" {
		t.Fatalf("unknown must omit $0: %+v", v)
	}
	if v.UnitPrices.Miss != nil || v.UnitPrices.CacheRead != nil ||
		v.UnitPrices.CacheCreate != nil || v.UnitPrices.Output != nil {
		t.Fatalf("unknown unit prices must be nil: %+v", v.UnitPrices)
	}
}

func TestByModelCreateFreeCard(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "zai", Vendor: "zhipu", Model: "glm-5", RequestID: "a",
			Miss: 1_000_000, CacheRead: 1_000_000, CacheCreate: 500_000, Output: 1_000_000},
	}
	sum := Aggregate(events, nil)
	row := modelRow(sum.ByModel, "zhipu", "glm-5")
	if row == nil {
		t.Fatalf("no glm-5 row %+v", sum.ByModel)
	}
	// glm-5 card: 1.0 / 0.20 / 0 (CreateFree) / 3.2 USD per 1M.
	if row.CostStatus != "complete" || row.CostMicro != 4_400_000 {
		t.Fatalf("glm-5 cost %+v", row)
	}
	v := ViewModel(*row)
	if v.Vendor != "zhipu" {
		t.Fatalf("vendor %q", v.Vendor)
	}
	up := v.UnitPrices
	if up.Miss == nil || *up.Miss != 1.0 {
		t.Fatalf("miss unit %+v", up.Miss)
	}
	if up.CacheRead == nil || *up.CacheRead != 0.2 {
		t.Fatalf("cache_read unit %+v", up.CacheRead)
	}
	if up.CacheCreate == nil || *up.CacheCreate != 0 {
		t.Fatalf("free cache_create must be non-nil 0: %+v", up.CacheCreate)
	}
	if up.Output == nil || *up.Output != 3.2 {
		t.Fatalf("output unit %+v", up.Output)
	}
}

func TestViewModelUnlistedComponentNil(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "codex", Vendor: "openai", Model: "gpt-5", RequestID: "a", Miss: 1_000_000, Output: 100_000},
	}
	sum := Aggregate(events, nil)
	row := modelRow(sum.ByModel, "openai", "gpt-5")
	if row == nil {
		t.Fatalf("no gpt-5 row %+v", sum.ByModel)
	}
	v := ViewModel(*row)
	if v.UnitPrices.CacheCreate != nil {
		t.Fatalf("unlisted cache_create must be nil: %+v", v.UnitPrices)
	}
	if v.UnitPrices.Miss == nil || *v.UnitPrices.Miss != 1.25 {
		t.Fatalf("miss unit %+v", v.UnitPrices.Miss)
	}
	if v.UnitPrices.CacheRead == nil || *v.UnitPrices.CacheRead != 0.125 {
		t.Fatalf("cache_read unit %+v", v.UnitPrices.CacheRead)
	}
	if v.UnitPrices.Output == nil || *v.UnitPrices.Output != 10 {
		t.Fatalf("output unit %+v", v.UnitPrices.Output)
	}
	if v.CostUSD != "$2.2500" || v.CostStatus != "complete" {
		t.Fatalf("view %+v", v)
	}
}
