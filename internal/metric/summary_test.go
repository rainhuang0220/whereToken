package metric

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestAggregateSplitsToolAndVendor(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Miss: 100, CacheRead: 900, Output: 10, Quality: event.QualityDegraded},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Miss: 50, Output: 5, Quality: event.QualityDegraded},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Miss: 20, CacheRead: 80, Output: 3, Quality: event.QualityAuthoritative},
	}
	turns := []event.TurnEvent{
		{Source: "claude"},
		{Source: "claude"},
		{Source: "kimi"},
	}
	sum := Aggregate(events, turns)
	if sum.All.Total() != 100+900+10+50+5+20+80+3 {
		t.Fatalf("all=%d", sum.All.Total())
	}
	var src, vend int64
	for _, s := range sum.BySource {
		src += s.Total()
		if s.ID == "claude" && s.UserTurns != 2 {
			t.Fatalf("claude turns=%d", s.UserTurns)
		}
	}
	for _, s := range sum.ByVendor {
		vend += s.Total()
	}
	if src != sum.All.Total() || vend != sum.All.Total() {
		t.Fatalf("conservation src=%d vend=%d all=%d", src, vend, sum.All.Total())
	}
	if sum.All.UserTurns != 3 {
		t.Fatalf("turns=%d", sum.All.UserTurns)
	}
	if len(sum.BySourceVendor) < 3 {
		t.Fatalf("cross=%d", len(sum.BySourceVendor))
	}
}

func TestAggregateDedupesRequestID(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "same", Miss: 1, CacheRead: 10, Output: 1},
		{Source: "claude", Vendor: "anthropic", RequestID: "same", Miss: 5, CacheRead: 10, Output: 2},
	}
	sum := Aggregate(events, nil)
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d", sum.All.Requests)
	}
	if sum.All.Miss != 5 || sum.All.Output != 2 || sum.All.CacheRead != 10 {
		t.Fatalf("max fields %+v", sum.All)
	}
}

func TestMergeComplementaryFieldsKeepsBoth(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", RequestID: "r", Miss: 100, Output: 0},
		{Source: "claude", RequestID: "r", Miss: 0, Output: 500},
	}
	sum := Aggregate(events, nil)
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d", sum.All.Requests)
	}
	if sum.All.Miss != 100 || sum.All.Output != 500 {
		t.Fatalf("complementary merge must keep both fields: %+v", sum.All)
	}
}

func TestMergeOutOfOrderAndMissingFields(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", RequestID: "r", Output: 4, CacheRead: 9000},
		{Source: "claude", RequestID: "r", Miss: 10, CacheRead: 9000},
		{Source: "claude", RequestID: "r", Miss: 0, Output: 1, CacheRead: 9000},
	}
	sum := Aggregate(events, nil)
	if sum.All.Miss != 10 || sum.All.Output != 4 || sum.All.CacheRead != 9000 {
		t.Fatalf("max per field %+v", sum.All)
	}
}

func TestAggregateRecordsAndDerivation(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", RequestID: "a", Miss: 1, Quality: event.QualityDegraded, Derivation: event.DeriveDeduplicated},
		{Source: "claude", RequestID: "a", Miss: 2, Quality: event.QualityDegraded, Derivation: event.DeriveDeduplicated},
		{Source: "kimi", RequestID: "b", Miss: 3, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw},
	}
	sum := Aggregate(events, nil)
	if sum.All.Records != 2 {
		t.Fatalf("records=%d", sum.All.Records)
	}
	for _, s := range sum.BySource {
		if s.ID == "claude" && (s.Records != 1 || s.Derivation != event.DeriveDeduplicated) {
			t.Fatalf("claude %+v", s)
		}
		if s.ID == "kimi" && (s.Records != 1 || s.Derivation != event.DeriveRaw) {
			t.Fatalf("kimi %+v", s)
		}
	}
}

func TestAggregateCostKnownModel(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Miss: 1_000_000, Output: 1_000_000, Quality: event.QualityDegraded},
	}
	sum := Aggregate(events, nil)
	if sum.All.CostStatus != "complete" || sum.All.CostMicro != 30_000_000 {
		t.Fatalf("cost %+v", sum.All)
	}
	v := View(sum.All)
	if v.CostUSD != "$30.0000" || v.CostStatus != "complete" {
		t.Fatalf("view %+v", v)
	}
}

func TestAggregateUnknownCostNotZeroUSD(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "a", Miss: 100, Output: 10, Quality: event.QualityAuthoritative},
	}
	sum := Aggregate(events, nil)
	if sum.All.CostStatus != "unavailable" || sum.All.CostMicro != 0 {
		t.Fatalf("%+v", sum.All)
	}
	v := View(sum.All)
	if v.CostUSD != "" || v.CostStatus != "unavailable" {
		t.Fatalf("must omit $0: %+v", v)
	}
}

func TestViewPartialZeroMicroOmitsUSD(t *testing.T) {
	s := Slice{CostStatus: "partial", CostMicro: 0, PricedTokens: 1, UnpricedTokens: 10, Miss: 11}
	v := View(s)
	if v.CostUSD != "" || v.MissCostUSD != "" {
		t.Fatalf("partial with no priced dollars must not print $0: %+v", v)
	}
	if v.CostStatus != "partial" {
		t.Fatalf("status=%s", v.CostStatus)
	}
}

func TestSourceVendorCostOmitsUnavailable(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Miss: 1_000_000, Output: 1_000_000},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", Miss: 100, Output: 10},
	}, nil)
	var claude, kimi *SourceVendor
	for i := range sum.BySourceVendor {
		row := &sum.BySourceVendor[i]
		switch row.Source {
		case "claude":
			claude = row
		case "kimi":
			kimi = row
		}
	}
	if claude == nil || claude.CostStatus != "complete" || claude.CostMicro != 30_000_000 {
		t.Fatalf("claude cross %+v", claude)
	}
	if kimi == nil || kimi.CostStatus != "unavailable" || kimi.CostMicro != 0 {
		t.Fatalf("kimi cross %+v", kimi)
	}
	if FormatCostUSD(kimi.CostStatus, kimi.CostMicro) != "" {
		t.Fatal("kimi cross must omit $0")
	}
	if FormatCostUSD(claude.CostStatus, claude.CostMicro) != "$30.0000" {
		t.Fatalf("claude %s", FormatCostUSD(claude.CostStatus, claude.CostMicro))
	}
}

func TestAggregatePartialCost(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Miss: 1_000_000},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", Miss: 1_000_000},
	}
	sum := Aggregate(events, nil)
	if sum.All.CostStatus != "partial" || sum.All.CostMicro != 5_000_000 || sum.All.UnpricedTokens != 1_000_000 {
		t.Fatalf("%+v", sum.All)
	}
}

func TestAggregateSkipRequestKeepsTokenTotals(t *testing.T) {
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "anthropic", RequestID: "bubble", Quality: event.QualityDegraded},
		{Source: "cursor", Vendor: "anthropic", RequestID: "api-1", Miss: 40, CacheRead: 200, CacheCreate: 10, Output: 5, Quality: event.QualityAuthoritative, SkipRequest: true},
	}
	sum := Aggregate(events, nil)
	if sum.All.Requests != 1 {
		t.Fatalf("requests=%d want 1 (API row must not count as a request)", sum.All.Requests)
	}
	if sum.All.Total() != 255 {
		t.Fatalf("total=%d", sum.All.Total())
	}
	if sum.BySourceVendor[0].Requests != 1 {
		t.Fatalf("cross requests=%d", sum.BySourceVendor[0].Requests)
	}
}
