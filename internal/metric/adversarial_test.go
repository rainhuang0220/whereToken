package metric

import (
	"math"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestNegativeTokensDoNotInventUsage(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "claude", RequestID: "n", Miss: -5, Output: -2, CacheRead: -1},
		{Source: "claude", RequestID: "ok", Miss: 10, Output: 1},
	}, nil)
	if sum.All.Total() < 0 {
		t.Fatalf("negative tokens leaked into total %+v", sum.All)
	}
	if sum.All.Miss != 10 || sum.All.Output != 1 || sum.All.Requests != 1 {
		t.Fatalf("good sibling dropped: %+v", sum.All)
	}
}

func TestNegativeReasoningDoesNotDropPositiveTokens(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{{
		Source: "grok", RequestID: "g", Miss: 100, Output: 10, Reasoning: -1,
	}}, nil)
	if sum.All.Miss != 100 || sum.All.Output != 10 || sum.All.Total() != 110 {
		t.Fatalf("display-only reasoning must not drop billed tokens: %+v", sum.All)
	}
}

func TestEmptyModelStaysUnpriced(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{{
		Source: "claude", Vendor: "anthropic", Model: "", Miss: 1000, Output: 10,
	}}, nil)
	if sum.All.CostStatus == "complete" && sum.All.CostMicro > 0 {
		t.Fatalf("empty model must not pick a card: %+v", sum.All)
	}
}

func TestUnknownProviderIsNotZeroDollars(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{{
		Source: "qwen", Vendor: "unknown", Model: "mystery", Miss: 1_000_000, Output: 1_000_000,
	}}, nil)
	v := View(sum.All)
	if v.CostUSD != "" || v.CostStatus == "complete" {
		t.Fatalf("unknown must omit USD: %+v", v)
	}
}

func TestDuplicateRequestIDTakesMaxNotSum(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "gemini", RequestID: "g1", Miss: 10, Output: 1},
		{Source: "gemini", RequestID: "g1", Miss: 12, Output: 0},
	}, nil)
	if sum.All.Miss != 12 || sum.All.Output != 1 || sum.All.Requests != 1 {
		t.Fatalf("merge %+v", sum.All)
	}
}

func TestWindowExcludesUndatedWhenBounded(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 8, 20, 12, 0, 0, 0, loc)
	w, err := ParseWindow(true, "", "", "", now, loc)
	if err != nil {
		t.Fatal(err)
	}
	if w.Contains(time.Time{}, loc) {
		t.Fatal("bounded window must drop undated events")
	}
	if !w.Contains(now, loc) {
		t.Fatal("today must include now")
	}
}

func TestHugeTokenSumDoesNotOverflowNegative(t *testing.T) {
	sum := Aggregate([]event.UsageEvent{
		{Source: "claude", RequestID: "a", Miss: math.MaxInt64 - 5, Output: 1},
		{Source: "claude", RequestID: "b", Miss: 10, Output: 1},
	}, nil)
	if sum.All.Total() < 0 {
		t.Fatalf("overflowed to negative %+v", sum.All)
	}
	if sum.All.Miss != math.MaxInt64 {
		t.Fatalf("want saturated miss, got %d", sum.All.Miss)
	}
}
