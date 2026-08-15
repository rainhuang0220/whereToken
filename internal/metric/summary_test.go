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
