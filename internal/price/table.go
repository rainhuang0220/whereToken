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
	anth("haiku-4", 1, 0.10, 1.25, 5),

	// xAI — docs.x.ai/developers/pricing short-context tier (2026-08-19).
	// Long-context rates (≥200k prompt) are not applied; that would need
	// per-request prompt length, which the ledger does not store.
	xai("grok-4.6", 2, 0.50, 0, 6),
	xai("grok-4.5", 2, 0.30, 0, 6),
	xai("grok-4.3", 1.25, 0.20, 0, 2.50),
	xai("grok-build", 1, 0.20, 0, 2),
	xai("grok-4", 2, 0.50, 0, 6),

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
	oai("o4-mini", 0.55, 0.28, 0, 2.20),
	oai("o3-mini", 1.10, 0.55, 0, 4.40),
	oai("o3", 2.00, 0.50, 0, 8),
}

func anth(model string, miss, cacheRead, cacheCreate, output float64) Rate {
	return Rate{
		Vendor: "anthropic", Model: model,
		Miss: miss, CacheRead: cacheRead, CacheCreate: cacheCreate, Output: output,
		From:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source: "anthropic_api_list", Version: CardVersion,
	}
}

func xai(model string, miss, cacheRead, cacheCreate, output float64) Rate {
	return Rate{
		Vendor: "xai", Model: model,
		Miss: miss, CacheRead: cacheRead, CacheCreate: cacheCreate, Output: output,
		From:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source: "xai_api_list", Version: CardVersion,
	}
}

func oai(model string, miss, cacheRead, cacheCreate, output float64) Rate {
	return Rate{
		Vendor: "openai", Model: model,
		Miss: miss, CacheRead: cacheRead, CacheCreate: cacheCreate, Output: output,
		From:   time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC),
		Source: "openai_api_list", Version: CardVersion,
	}
}
