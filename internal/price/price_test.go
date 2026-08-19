package price

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestKimiK3UsesOfficialCardButBareK3StaysUnpriced(t *testing.T) {
	k3 := Event(event.UsageEvent{Vendor: "moonshot", Model: "k3", Miss: 1_000_000, Output: 1_000_000})
	if k3.OK {
		t.Fatal("bare k3 must stay unpriced")
	}
	full := Event(event.UsageEvent{Vendor: "moonshot", Model: "kimi-k3", Miss: 1_000_000, Output: 1_000_000})
	if !full.OK || full.Micro != 18_000_000 { // $3+$15
		t.Fatalf("kimi-k3 %+v", full)
	}
	hs := Event(event.UsageEvent{Vendor: "moonshot", Model: "kimi-k2.7-code-highspeed", Miss: 1_000_000, Output: 1_000_000})
	base := Event(event.UsageEvent{Vendor: "moonshot", Model: "kimi-k2.7-code", Miss: 1_000_000, Output: 1_000_000})
	if !hs.OK || hs.Micro != 9_900_000 { // $1.90+$8
		t.Fatalf("highspeed %+v", hs)
	}
	if !base.OK || base.Micro != 4_950_000 { // $0.95+$4
		t.Fatalf("k2.7-code %+v", base)
	}
}

func TestHaiku4StaysUnpriced(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-haiku-4", Miss: 1_000_000, Output: 1_000_000})
	if c.OK {
		t.Fatal("there is no Haiku 4 list row; do not copy 4.5 rates")
	}
}

func TestGrokBuild01UsesOfficialSlug(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "xai", Model: "grok-build-0.1", Miss: 1_000_000, Output: 1_000_000})
	if !c.OK || c.Micro != 3_000_000 { // $1+$2
		t.Fatalf("grok-build-0.1 %+v", c)
	}
}

func TestGLM53DoesNotInheritGLM5(t *testing.T) {
	v53 := Event(event.UsageEvent{Vendor: "zhipu", Model: "glm-5.3", Miss: 1_000_000, Output: 1_000_000})
	v5 := Event(event.UsageEvent{Vendor: "zhipu", Model: "glm-5", Miss: 1_000_000, Output: 1_000_000})
	if !v53.OK || v53.Micro != 5_800_000 { // $1.4+$4.4
		t.Fatalf("glm-5.3 %+v", v53)
	}
	if !v5.OK || v5.Micro != 4_200_000 { // $1+$3.2
		t.Fatalf("glm-5 %+v", v5)
	}
}

func TestGeminiFlashPricedProUnpriced(t *testing.T) {
	flash := Event(event.UsageEvent{Vendor: "google", Model: "gemini-2.5-flash", Miss: 1_000_000, Output: 1_000_000})
	if !flash.OK || flash.Micro != 2_800_000 { // $0.30+$2.50
		t.Fatalf("flash %+v", flash)
	}
	lite := Event(event.UsageEvent{Vendor: "google", Model: "gemini-2.5-flash-lite", Miss: 1_000_000, Output: 1_000_000})
	if !lite.OK || lite.Micro != 500_000 { // $0.10+$0.40
		t.Fatalf("lite must not inherit flash: %+v", lite)
	}
	pro := Event(event.UsageEvent{Vendor: "google", Model: "gemini-2.5-pro", Miss: 1_000_000, Output: 1_000_000})
	if pro.OK {
		t.Fatal("gemini-2.5-pro is context-tiered and must stay unpriced")
	}
}

func TestUnavailableNeverFormatsAsZeroUSD(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "moonshot", Model: "k3", Miss: 1_000_000, Output: 1_000_000})
	if c.OK {
		t.Fatal("moonshot must stay unpriced")
	}
	if FormatUSD(c.Micro) == "$0.0000" {
		t.Fatal("unavailable must not format as $0.0000")
	}
	if FormatUSD(c.Micro) != "" {
		t.Fatalf("unavailable format %q", FormatUSD(c.Micro))
	}
}

func TestHyphenOpus46UsesCurrentCardNotRetiredOpus4(t *testing.T) {
	hyphen := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4-6", Miss: 1_000_000, Output: 1_000_000})
	dotted := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Output: 1_000_000})
	if !hyphen.OK || hyphen.Micro != dotted.Micro || hyphen.Micro != 30_000_000 {
		t.Fatalf("Cursor/API id claude-opus-4-6 must use $5+$25, not retired opus-4: hyphen=%+v dotted=%+v", hyphen, dotted)
	}
}

func TestGrok4BareAndHyphenSlugsStayUnpriced(t *testing.T) {
	for _, model := range []string{"grok-4", "grok-4-fast", "grok-4-latest"} {
		c := Event(event.UsageEvent{Vendor: "xai", Model: model, Miss: 1_000_000, Output: 1_000_000})
		if c.OK {
			t.Fatalf("%s must stay unpriced (no grok-4 list row): %+v", model, c)
		}
		if FormatUSD(c.Micro) != "" {
			t.Fatalf("%s unknown cost must omit $0.0000, got %q", model, FormatUSD(c.Micro))
		}
	}
}

func TestGrok46AndBuildStayOnShortContextCard(t *testing.T) {
	for _, model := range []string{"grok-4.6", "grok-4.6-build"} {
		c := Event(event.UsageEvent{Vendor: "xai", Model: model, Miss: 1_000_000, Output: 1_000_000})
		if !c.OK || c.Micro != 8_000_000 { // $2+$6
			t.Fatalf("%s must stay $2+$6: %+v", model, c)
		}
	}
}

func TestGPT5MiniUsesOwnCardNotGPT5(t *testing.T) {
	mini := Event(event.UsageEvent{Vendor: "openai", Model: "gpt-5-mini", Miss: 1_000_000, Output: 1_000_000})
	full := Event(event.UsageEvent{Vendor: "openai", Model: "gpt-5", Miss: 1_000_000, Output: 1_000_000})
	if !mini.OK || mini.Micro != 2_250_000 { // $0.25+$2
		t.Fatalf("gpt-5-mini must use its own card: %+v", mini)
	}
	if !full.OK || full.Micro != 11_250_000 { // $1.25+$10
		t.Fatalf("gpt-5 %+v", full)
	}
	if mini.Micro == full.Micro {
		t.Fatal("gpt-5-mini must not inherit gpt-5")
	}
}

func TestOpus48DoesNotInheritRetiredOpus4(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4-8", Miss: 1_000_000, Output: 1_000_000})
	if !c.OK || c.Micro != 30_000_000 {
		t.Fatalf("opus-4.8 must not inherit opus-4 $15+$75: %+v", c)
	}
}

func TestShippedTableBackdatesOpenCard(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "anthropic", Model: "claude-opus-4.6",
		Miss: 1_000_000, Output: 1_000_000,
		Timestamp: time.Date(2025, 6, 1, 0, 0, 0, 0, time.UTC),
	})
	if !c.OK || c.Version != CardVersion || c.Micro != 30_000_000 {
		t.Fatalf("2025 timestamp uses the open %s card, not a 2025 invoice: %+v", CardVersion, c)
	}
}

func TestO4MiniUsesStandardListNotBatch(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "openai", Model: "o4-mini",
		Miss: 1_000_000, CacheRead: 1_000_000, Output: 1_000_000,
	})
	// Standard list $1.10 / $0.275 / $4.40 — not Batch/Flex $0.55 / $2.20.
	if !c.OK || c.Micro != 5_775_000 {
		t.Fatalf("o4-mini must use standard list, got %+v", c)
	}
	if c.Miss != 1_100_000 || c.CacheRead != 275_000 || c.Output != 4_400_000 {
		t.Fatalf("components %+v", c)
	}
}

func TestChatGPT4oDoesNotInheritGPT4o(t *testing.T) {
	for _, model := range []string{"chatgpt-4o", "chatgpt-5", "foo-o3"} {
		c := Event(event.UsageEvent{Vendor: "openai", Model: model, Miss: 1_000_000, Output: 1_000_000})
		if c.OK {
			t.Fatalf("%s must not inherit a sibling list id: %+v", model, c)
		}
	}
}

func TestGrokCacheCreateWithoutWriteRateIsUnpriced(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "xai", Model: "grok-4.6",
		CacheCreate: 1_000_000,
	})
	if c.OK {
		t.Fatalf("xAI has no public cache-write rate; must not be complete $0: %+v", c)
	}
	if FormatUSD(c.Micro) != "" {
		t.Fatalf("unpriced cache write formatted %q", FormatUSD(c.Micro))
	}
	priced := Event(event.UsageEvent{
		Vendor: "xai", Model: "grok-4.6",
		Miss: 1_000_000, Output: 1_000_000,
	})
	if !priced.OK || priced.Micro != 8_000_000 {
		t.Fatalf("Grok without cache write still prices input/output: %+v", priced)
	}
}

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
	grok := Event(event.UsageEvent{
		Vendor: "xai", Model: "grok-4.6-build",
		Output: 1_000_000, Reasoning: 1_000_000,
	})
	if !grok.OK || grok.Micro != 6_000_000 {
		t.Fatalf("Grok reasoning is not a second output line %+v", grok)
	}
	mm := Event(event.UsageEvent{
		Vendor: "minimax", Model: "MiniMax-M2.5",
		Output: 1_000_000, Reasoning: 1_000_000,
	})
	if !mm.OK || mm.Micro != 1_200_000 {
		t.Fatalf("MiniMax reasoning is not a second output line %+v", mm)
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

func TestOpus4RetiredCardNotCurrentOpus(t *testing.T) {
	old := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4", Miss: 1_000_000, Output: 1_000_000})
	cur := Event(event.UsageEvent{Vendor: "anthropic", Model: "claude-opus-4.6", Miss: 1_000_000, Output: 1_000_000})
	if !old.OK || old.Micro != 90_000_000 { // $15 + $75
		t.Fatalf("opus-4 retired %+v", old)
	}
	if !cur.OK || cur.Micro != 30_000_000 { // $5 + $25
		t.Fatalf("opus-4.6 must not inherit opus-4 rates %+v", cur)
	}
}

func TestMiniMaxM21ListPrice(t *testing.T) {
	c := Event(event.UsageEvent{
		Vendor: "minimax", Model: "MiniMax-M2.1",
		Miss: 1_000_000, CacheRead: 1_000_000, CacheCreate: 1_000_000, Output: 1_000_000,
	})
	if !c.OK || c.Micro != 1_905_000 { // 0.30+0.03+0.375+1.20
		t.Fatalf("m2.1 %+v", c)
	}
	fast := Event(event.UsageEvent{Vendor: "minimax", Model: "MiniMax-M2.1-highspeed", Miss: 1_000_000})
	if !fast.OK || fast.Micro != 600_000 {
		t.Fatalf("highspeed must not inherit the cheap card %+v", fast)
	}
	m27 := Event(event.UsageEvent{Vendor: "minimax", Model: "MiniMax-M2.7", CacheRead: 1_000_000})
	if !m27.OK || m27.Micro != 60_000 {
		t.Fatalf("m2.7 cache read is $0.06 %+v", m27)
	}
}

func TestMiniMaxM3StaysUnavailable(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "minimax", Model: "minimax/MiniMax-M3", Miss: 1_000_000, Output: 1_000_000})
	if c.OK {
		t.Fatal("M3 context-tiered list must not invent one rate")
	}
}

func TestGPT56CacheWriteIsPriced(t *testing.T) {
	c := Event(event.UsageEvent{Vendor: "openai", Model: "gpt-5.6-sol", CacheCreate: 1_000_000})
	if !c.OK || c.Micro != 6_250_000 {
		t.Fatalf("sol cache write %+v", c)
	}
	terra := Event(event.UsageEvent{Vendor: "openai", Model: "gpt-5.6-terra", CacheCreate: 1_000_000})
	if !terra.OK || terra.Micro != 2_500_000 {
		t.Fatalf("terra cache write %+v", terra)
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
	if FormatUSD(0) != "" {
		t.Fatalf("zero %q", FormatUSD(0))
	}
	if FormatUSD(1) != "" {
		t.Fatalf("rounds to $0.0000 must omit %q", FormatUSD(1))
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
