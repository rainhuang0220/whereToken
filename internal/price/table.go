package price

import "time"

// table is the public list-price card. Values are USD per million tokens.
// Sources are the vendor API list pages as of CardVersion, not invoices.
//
// Limitation: this tree does not contain a full historical archive. Events
// with a timestamp use the open-ended 2026-08-19 card (From = 2024-01-01).
// A later card with a closed To would leave older events on this one.
var table = []Rate{
	// Anthropic — platform.claude.com/docs/en/about-claude/pricing (2026-08-19)
	// Cache write = 5-minute cache write. 1-hour writes are not in the ledger.
	anth("fable-5", 10, 1.00, 12.50, 50),
	anth("mythos-5", 10, 1.00, 12.50, 50),
	anth("opus-4.8", 5, 0.50, 6.25, 25),
	anth("opus-4.7", 5, 0.50, 6.25, 25),
	anth("opus-4.6", 5, 0.50, 6.25, 25),
	anth("opus-4.5", 5, 0.50, 6.25, 25),
	anth("opus-5", 5, 0.50, 6.25, 25),
	anth("opus-4.1", 15, 1.50, 18.75, 75),
	anth("opus-4", 15, 1.50, 18.75, 75),
	anth("sonnet-4.6", 3, 0.30, 3.75, 15),
	anth("sonnet-4.5", 3, 0.30, 3.75, 15),
	anth("sonnet-5", 2, 0.20, 2.50, 10),
	anth("sonnet-4", 3, 0.30, 3.75, 15),
	anth("haiku-4.5", 1, 0.10, 1.25, 5),

	// xAI — docs.x.ai/developers/pricing short-context tier (2026-08-19).
	// Long-context rates (≥200k prompt) are not applied; that would need
	// per-request prompt length, which the ledger does not store.
	// No grok-4 row: the public list has none. grok-4-0709 retired 2026-05-15;
	// leftover slugs are not $2/$6 on this card.
	// grok-4.6-build is the Grok CLI product slug, not a separate list row.
	// It uses the public grok-4.6 short-context rates.
	xai("grok-4.6-build", 2, 0.50, 0, 6),
	xai("grok-4.6", 2, 0.50, 0, 6),
	xai("grok-4.5", 2, 0.30, 0, 6),
	xai("grok-4.3", 1.25, 0.20, 0, 2.50),
	xai("grok-build-0.1", 1, 0.20, 0, 2),

	// OpenAI — developers.openai.com/api/docs/pricing short-context + long-standing 4o card
	oai("gpt-4o-mini", 0.15, 0.075, 0, 0.60),
	oai("gpt-4o", 2.50, 1.25, 0, 10),
	oai("gpt-4.1-mini", 0.40, 0.10, 0, 1.60),
	oai("gpt-4.1", 2.00, 0.50, 0, 8),
	oai("gpt-5.6-sol", 5.00, 0.50, 6.25, 30),
	oai("gpt-5.6-terra", 2.00, 0.20, 2.50, 12),
	oai("gpt-5.6-luna", 0.20, 0.02, 0.25, 1.20),
	oai("gpt-5.3-codex", 1.75, 0.175, 0, 14),
	oai("gpt-5-mini", 0.25, 0.025, 0, 2),
	oai("gpt-5-nano", 0.05, 0.005, 0, 0.40),
	oai("gpt-5", 1.25, 0.125, 0, 10),
	oai("o4-mini", 1.10, 0.275, 0, 4.40),
	oai("o3-mini", 1.10, 0.55, 0, 4.40),
	oai("o3", 2.00, 0.50, 0, 8),

	// MiniMax — platform.minimax.io/docs/guides/pricing-paygo (2026-08-19)
	// International API list. Token Plan subscriptions are not this card.
	// M3 is not listed here: the public page splits ≤512k / >512k and prints
	// strikethrough promo pairs. Guessing one rate would be a billed fiction.
	// Moonshot — platform.kimi.ai/docs/pricing (2026-08-20). No cache-write
	// token rate; events with CacheCreate>0 stay unpriced.
	kimi("kimi-k3", 3.00, 0.30, 0, 15.00),
	kimi("kimi-k2.7-code-highspeed", 1.90, 0.38, 0, 8.00),
	kimi("kimi-k2.7-code", 0.95, 0.19, 0, 4.00),
	kimi("kimi-k2.6", 0.95, 0.16, 0, 4.00),
	kimi("kimi-k2.5", 0.60, 0.10, 0, 3.00),

	// Z.ai — docs.z.ai/guides/overview/pricing (2026-08-25). Cache write
	// storage is listed as limited-time free, so CacheCreate bills $0
	// (CreateFree). The GLM-*.7/4.5-Flash rows are list-price free and stay
	// unpriced here so they never render as $0.
	zhi("glm-5.3", 1.4, 0.26, 0, 4.4),
	zhi("glm-5.2", 1.4, 0.26, 0, 4.4),
	zhi("glm-5.1", 1.4, 0.26, 0, 4.4),
	zhi("glm-5-turbo", 1.2, 0.24, 0, 4.0),
	zhi("glm-5", 1.0, 0.20, 0, 3.2),
	zhi("glm-4.7", 0.6, 0.11, 0, 2.2),
	zhi("glm-4.7-flashx", 0.07, 0.01, 0, 0.4),
	zhi("glm-4.6", 0.6, 0.11, 0, 2.2),
	zhi("glm-4.5", 0.6, 0.11, 0, 2.2),
	zhi("glm-4.5-x", 2.2, 0.45, 0, 8.9),
	zhi("glm-4.5-air", 0.2, 0.03, 0, 1.1),
	zhi("glm-4.5-airx", 1.1, 0.22, 0, 4.5),

	// DeepSeek — api-docs.deepseek.com/quick_start/pricing (2026-08-25).
	// Peak rates. Off-peak (01:00-04:00, 06:00-10:00 UTC Mon-Fri) is half;
	// that time-of-day split is not applied (same class of approximation as
	// the xAI long-context note). Context caching has no write charge.
	ds("deepseek-v4-flash", 0.44, 0.014, 0, 1.32),
	ds("deepseek-v4-flash-vision-exp", 0.44, 0.014, 0, 1.32),
	ds("deepseek-v4-pro", 1.32, 0.044, 0, 3.96),

	// Google — ai.google.dev/gemini-api/docs/pricing (2026-08-13). Flat
	// Flash/Lite rows only. 2.5 Pro / 3.x Pro stay unpriced (≤200k / >200k).
	// Cache write is hourly storage, not a token rate.
	ggl("gemini-2.5-flash-lite", 0.10, 0.01, 0, 0.40),
	ggl("gemini-2.5-flash", 0.30, 0.03, 0, 2.50),
	ggl("gemini-2.0-flash-lite", 0.075, 0, 0, 0.30),
	ggl("gemini-2.0-flash", 0.10, 0.025, 0, 0.40),
	ggl("gemini-3.5-flash-lite", 0.30, 0.03, 0, 2.50),
	ggl("gemini-3.5-flash", 1.50, 0.15, 0, 9.00),

	mm("minimax-m2.7-highspeed", 0.60, 0.06, 0.375, 2.40),
	mm("minimax-m2.7", 0.30, 0.06, 0.375, 1.20),
	mm("minimax-m2.5-highspeed", 0.60, 0.03, 0.375, 2.40),
	mm("minimax-m2.5", 0.30, 0.03, 0.375, 1.20),
	mm("minimax-m2.1-highspeed", 0.60, 0.03, 0.375, 2.40),
	mm("minimax-m2.1", 0.30, 0.03, 0.375, 1.20),
}

// card returns a list-price row constructor pinned to one vendor's public
// API list. All rows share the same open-ended validity window.
func card(vendor, source string) func(model string, miss, cacheRead, cacheCreate, output float64) Rate {
	return cardOpt(vendor, source, false)
}

// cardOpt is card with the cache-write-is-free flag some cards declare.
func cardOpt(vendor, source string, createFree bool) func(model string, miss, cacheRead, cacheCreate, output float64) Rate {
	return func(model string, miss, cacheRead, cacheCreate, output float64) Rate {
		return Rate{
			Vendor: vendor, Model: model,
			Miss: miss, CacheRead: cacheRead, CacheCreate: cacheCreate, Output: output,
			From:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
			Source: source, Version: CardVersion,
			CreateFree: createFree,
		}
	}
}

var (
	anth = card("anthropic", "anthropic_api_list")
	xai  = card("xai", "xai_api_list")
	oai  = card("openai", "openai_api_list")
	mm   = card("minimax", "minimax_api_list")
	kimi = card("moonshot", "moonshot_api_list")
	ggl  = card("google", "google_api_list")
	// Z.ai lists cache-write storage as limited-time free: bill it $0.
	zhi = cardOpt("zhipu", "zai_api_list", true)
	ds  = card("deepseek", "deepseek_api_list")
)
