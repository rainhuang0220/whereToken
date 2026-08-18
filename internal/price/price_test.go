package price

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestUnknownModelHasNoCost(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "unknown", Model: "mystery", Miss: 1000, Output: 100})
	if c.OK {
		t.Fatal("unknown model must not invent a price")
	}
}

func TestUnknownVendorHasNoCost(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "acme", Model: "claude-opus-4.6", Miss: 100})
	if c.OK {
		t.Fatal("wrong vendor must not use another vendor's card")
	}
}

func TestOpusMissAndCacheAndOutput(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "anthropic", Model: "claude-opus-4.6",
		Miss: 1_000_000, CacheRead: 1_000_000, CacheCreate: 1_000_000, Output: 1_000_000,
	})
	if !c.OK {
		t.Fatal("expected rate")
	}
	// $5 + $0.50 + $6.25 + $25 = $36.75
	if c.Micro != 36_750_000 {
		t.Fatalf("micro=%d", c.Micro)
	}
	if c.Miss != 5_000_000 || c.CacheRead != 500_000 || c.CacheCreate != 6_250_000 || c.Output != 25_000_000 {
		t.Fatalf("%+v", c)
	}
}

func TestReasoningNotChargedTwice(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "openai", Model: "gpt-5",
		Output: 1_000_000, Reasoning: 1_000_000,
	})
	if !c.OK || c.Micro != 10_000_000 {
		t.Fatalf("output-only $10, got %+v", c)
	}
}

func TestCacheHeavyVsOutputHeavy(t *testing.T) {
	cache := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-haiku-4.5", CacheRead: 10_000_000})
	out := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-haiku-4.5", Output: 10_000_000})
	if !cache.OK || !out.OK {
		t.Fatal("haiku")
	}
	if cache.Micro >= out.Micro {
		t.Fatalf("cache $%d should be cheaper than output $%d", cache.Micro, out.Micro)
	}
}

func TestAliasPathAndUnderscore(t *testing.T) {
	a := Event(event.UsageEvent{Vendor: "anthropic", Model: "anthropic/claude-sonnet-5", Miss: 1_000_000})
	b := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude_sonnet_5", Miss: 1_000_000})
	if !a.OK || !b.OK || a.Micro != b.Micro || a.Micro != 2_000_000 {
		t.Fatalf("alias a=%+v b=%+v", a, b)
	}
}

func TestZeroTokensKnownModelIsZeroCost(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6"})
	if !c.OK || c.Micro != 0 {
		t.Fatalf("%+v", c)
	}
}

func TestGrokShortContextCard(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "xai", Model: "grok-4.6-build", Miss: 1_000_000, CacheRead: 1_000_000, Output: 1_000_000})
	if !c.OK || c.Micro != 8_500_000 { // 2+0.5+6
		t.Fatalf("%+v", c)
	}
}

func TestHistoricalWindow(t *testing.T) {
	old := Rate{
		Vendor: "anthropic", Model: "hist-only",
		Miss: 9, Output: 9,
		From:    time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC),
		To:      time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Version: "old",
	}
	cur := Rate{
		Vendor: "anthropic", Model: "hist-only",
		Miss: 1, Output: 1,
		From:    time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC),
		Version: "new",
	}
	prev := table
	table = []Rate{old, cur}
	defer func() { table = prev }()

	r, ok := Lookup("anthropic", "hist-only", time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC))
	if !ok || r.Version != "old" || r.Miss != 9 {
		t.Fatalf("old window %+v ok=%v", r, ok)
	}
	r, ok = Lookup("anthropic", "hist-only", time.Date(2026, 6, 1, 0, 0, 0, 0, time.UTC))
	if !ok || r.Version != "new" || r.Miss != 1 {
		t.Fatalf("new window %+v ok=%v", r, ok)
	}
	r, ok = Lookup("anthropic", "hist-only", time.Time{})
	if !ok || r.Version != "new" {
		t.Fatalf("undated must use open card %+v ok=%v", r, ok)
	}
}

func TestStatus(t *testing.T) {
	if Status(0, 0) != StatusUnavailable {
		t.Fatal("empty")
	}
	if Status(0, 10) != StatusUnavailable {
		t.Fatal("all unknown")
	}
	if Status(10, 0) != StatusComplete {
		t.Fatal("all priced")
	}
	if Status(10, 5) != StatusPartial {
		t.Fatal("mix")
	}
}

func TestFormatUSD(t *testing.T) {
	if FormatUSD(36_750_000) != "$36.7500" {
		t.Fatalf("%s", FormatUSD(36_750_000))
	}
}

func TestDuplicateMergeThenPrice(t *testing.T) {
	// caller must merge first; pricing a raw stream twice would be wrong.
	a := event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "r", Miss: 100, Output: 0}
	b := event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "r", Miss: 0, Output: 500}
	// if priced separately and summed: miss*5e-6*100 + out*25e-6*500
	// after max merge: miss 100 + out 500, same arithmetic as sum of exclusive fields
	ca, cb := Event(a), Event(b)
	merged := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 100, Output: 500})
	if merged.Micro != ca.Micro+cb.Micro {
		t.Fatalf("complementary %d vs %d+%d", merged.Micro, ca.Micro, cb.Micro)
	}
}
