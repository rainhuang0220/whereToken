package insight

import (
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

var evalNow = time.Date(2026, 8, 20, 12, 0, 0, 0, time.UTC)

// dayEvents builds one event per day ending at evalNow, so active-day rules
// see exactly len(days) active days.
func dayEvents(vendor, model string, totals ...int64) []event.UsageEvent {
	var out []event.UsageEvent
	for i, tot := range totals {
		out = append(out, event.UsageEvent{
			Source:    "claude",
			Vendor:    vendor,
			Model:     model,
			Miss:      tot,
			Timestamp: evalNow.AddDate(0, 0, -(len(totals) - 1 - i)),
		})
	}
	return out
}

func evalOf(events []event.UsageEvent) Evaluation {
	return Evaluate(metric.AggregateAt(events, nil, evalNow, time.UTC))
}

func TestEvaluateEmptyIsNoneNeverLight(t *testing.T) {
	got := evalOf(nil)
	if got.Level != LevelNone || got.Summary != "—" || got.Reason != "" {
		t.Fatalf("%+v", got)
	}
}

func TestEvaluateInsufficientBoundary(t *testing.T) {
	below := evalOf(dayEvents("unknown", "m-a", evalMinTokens-1))
	if below.Level != LevelInsufficient || below.Summary != "暂无评价" {
		t.Fatalf("below min tokens: %+v", below)
	}
	if !strings.Contains(below.Reason, "0.10") && !strings.Contains(below.Reason, "tokens") {
		t.Fatalf("insufficient must explain itself: %+v", below)
	}
	at := evalOf(dayEvents("unknown", "m-a", evalMinTokens))
	if at.Level == LevelInsufficient || at.Level == LevelNone {
		t.Fatalf("at min tokens there is a profile: %+v", at)
	}
}

func TestEvaluateHighCostBoundary(t *testing.T) {
	// opus-4.6 output list is $25 / 1M, so 1M output tokens is exactly
	// $25 on a single active day.
	at := evalOf([]event.UsageEvent{{
		Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
		Output: 1_000_000, Timestamp: evalNow,
	}})
	if at.Level != LevelHighCost || at.Summary != "成本偏高" {
		t.Fatalf("at $25/day: %+v", at)
	}
	if !strings.Contains(at.Reason, "$25.00") || !strings.Contains(at.Reason, "opus-4.6") {
		t.Fatalf("reason must cite the numbers and the model: %q", at.Reason)
	}
	below := evalOf([]event.UsageEvent{{
		Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6",
		Output: 999_999, Timestamp: evalNow,
	}})
	if below.Level == LevelHighCost {
		t.Fatalf("below $25/day: %+v", below)
	}
}

func TestEvaluateHighCostNeedsRealCost(t *testing.T) {
	// Unpriced usage is not cheap usage: unknown cost must skip the cost rule.
	got := evalOf(dayEvents("unknown", "m-a", 2_000_000))
	if got.Level == LevelHighCost {
		t.Fatalf("unknown price is not high cost: %+v", got)
	}
}

func TestEvaluateHighUsageBoundary(t *testing.T) {
	at := evalOf(dayEvents("moonshot", "k3", highUsagePerDayTokens))
	if at.Level != LevelHighUsage || at.Summary != "高强度使用" {
		t.Fatalf("at threshold: %+v", at)
	}
	if !strings.Contains(at.Reason, "5.00 M") || !strings.Contains(at.Reason, "Moonshot") {
		t.Fatalf("reason must cite tokens and vendor: %q", at.Reason)
	}
	below := evalOf(dayEvents("moonshot", "k3", highUsagePerDayTokens-1))
	if below.Level == LevelHighUsage {
		t.Fatalf("below threshold: %+v", below)
	}
}

func TestEvaluateMultiModelAndTopVendorBoundary(t *testing.T) {
	mk := func(a, b, c int64) []event.UsageEvent {
		return []event.UsageEvent{
			{Source: "kimi", Vendor: "moonshot", Model: "k3", Miss: a, Timestamp: evalNow},
			{Source: "qwen", Vendor: "alibaba", Model: "qwen3-coder-plus", Miss: b, Timestamp: evalNow},
			{Source: "x", Vendor: "doubao", Model: "seed-code", Miss: c, Timestamp: evalNow.AddDate(0, 0, -1)},
		}
	}
	got := evalOf(mk(500_000, 300_000, 200_000))
	if got.Level != LevelMultiModel || got.Summary != "多模型探索" {
		t.Fatalf("balanced 3 vendors: %+v", got)
	}
	if !strings.Contains(got.Reason, "3 个厂家") || !strings.Contains(got.Reason, "3 个模型") {
		t.Fatalf("reason must count vendors and models: %q", got.Reason)
	}
	// A 60% top vendor is not "较均衡".
	edge := evalOf(mk(600_000, 300_000, 100_000))
	if edge.Level == LevelMultiModel {
		t.Fatalf("60%% top vendor: %+v", edge)
	}
}

func TestEvaluateSingleModelBoundary(t *testing.T) {
	mk := func(a, b int64) []event.UsageEvent {
		return []event.UsageEvent{
			{Source: "s", Vendor: "unknown", Model: "m-a", Miss: a, Timestamp: evalNow},
			{Source: "s", Vendor: "unknown", Model: "m-b", Miss: b, Timestamp: evalNow},
		}
	}
	at := evalOf(mk(700_000, 300_000))
	if at.Level != LevelSingleModel || at.Summary != "单模型集中" {
		t.Fatalf("70%% top model: %+v", at)
	}
	if !strings.Contains(at.Reason, "m-a") || !strings.Contains(at.Reason, "70%") {
		t.Fatalf("reason must name the model and share: %q", at.Reason)
	}
	below := evalOf(mk(699_999, 300_001))
	if below.Level == LevelSingleModel {
		t.Fatalf("69.9999%% must not round up into the rule: %+v", below)
	}
}

func TestEvaluateCacheReuseBoundaries(t *testing.T) {
	mk := func(miss, crA, crB int64, days int) []event.UsageEvent {
		var out []event.UsageEvent
		for i := 0; i < days; i++ {
			ts := evalNow.AddDate(0, 0, -(days - 1 - i))
			m := int64(0)
			if i == 0 {
				m = miss
			}
			out = append(out,
				event.UsageEvent{Source: "s", Vendor: "unknown", Model: "m-a", Miss: m, CacheRead: crA / int64(days), Timestamp: ts},
				event.UsageEvent{Source: "s", Vendor: "unknown", Model: "m-b", CacheRead: crB / int64(days), Timestamp: ts},
			)
		}
		return out
	}
	// Exactly 75% cache read at 6M read tokens over two days.
	at := evalOf(mk(2_000_000, 3_500_000, 2_500_000, 2))
	if at.Level != LevelCacheReuse || at.Summary != "高缓存复用" {
		t.Fatalf("75%% hit at volume: %+v", at)
	}
	// One token less cache read: hit rate dips below 75%.
	belowHit := evalOf(mk(2_000_001, 3_500_000, 2_500_000, 2))
	if belowHit.Level == LevelCacheReuse {
		t.Fatalf("below 75%% hit: %+v", belowHit)
	}
	// High hit rate but under the volume floor is not called out.
	lowVolume := evalOf(mk(100_000, 260_000, 40_000, 2))
	if lowVolume.Level == LevelCacheReuse {
		t.Fatalf("under volume floor: %+v", lowVolume)
	}
}

func TestEvaluateSteadyBoundaries(t *testing.T) {
	mk := func(days int, perDay int64) []event.UsageEvent {
		var out []event.UsageEvent
		for i := 0; i < days; i++ {
			ts := evalNow.AddDate(0, 0, -(days - 1 - i))
			out = append(out,
				event.UsageEvent{Source: "s", Vendor: "unknown", Model: "m-a", Miss: perDay * 3 / 5, Timestamp: ts},
				event.UsageEvent{Source: "s", Vendor: "unknown", Model: "m-b", Miss: perDay * 2 / 5, Timestamp: ts},
			)
		}
		return out
	}
	at := evalOf(mk(steadyMinActiveDays, steadyMinPerDayTokens))
	if at.Level != LevelSteady || at.Summary != "稳定使用" {
		t.Fatalf("at steady floor: %+v", at)
	}
	fewerDays := evalOf(mk(steadyMinActiveDays-1, steadyMinPerDayTokens))
	if fewerDays.Level == LevelSteady {
		t.Fatalf("too few days: %+v", fewerDays)
	}
	lighter := evalOf(mk(steadyMinActiveDays, steadyMinPerDayTokens-1))
	if lighter.Level == LevelSteady {
		t.Fatalf("under daily floor: %+v", lighter)
	}
	if lighter.Level != LevelLight || lighter.Summary != "轻量使用" {
		t.Fatalf("steady misses fall to light: %+v", lighter)
	}
}

func TestEvaluateLightHonesty(t *testing.T) {
	unpriced := evalOf([]event.UsageEvent{
		{Source: "s", Vendor: "unknown", Model: "m-a", Miss: 60_000, Timestamp: evalNow},
		{Source: "s", Vendor: "unknown", Model: "m-b", Miss: 40_000, Timestamp: evalNow},
	})
	if unpriced.Level != LevelLight {
		t.Fatalf("%+v", unpriced)
	}
	if strings.Contains(unpriced.Reason, "成本较低") {
		t.Fatalf("unknown cost must not claim low cost: %q", unpriced.Reason)
	}
	priced := evalOf([]event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 60_000, Timestamp: evalNow},
		{Source: "claude", Vendor: "anthropic", Model: "claude-sonnet-4.6", Miss: 40_000, Timestamp: evalNow},
	})
	if priced.Level != LevelLight || !strings.Contains(priced.Reason, "成本较低") {
		t.Fatalf("priced tiny usage may say low cost: %+v", priced)
	}
}
