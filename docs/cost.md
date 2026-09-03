# Cost (API-equivalent estimate)

whereToken can attach a **USD estimate** to priced models. This is **not** a
provider invoice and **not** what a subscription or bundled credit actually
billed.

```text
priced event
    ↓
list price card (vendor + canonical model + timestamp window)
    ↓
miss_cost + cache_read_cost + cache_create_cost + output_cost
```

Reasoning is never charged as its own line. If an adapter already folded
reasoning into `Output` (Codex, OpenCode, Kilo CLI, ZCode), only `Output`
is priced. Grok and MiniMax store reasoning beside output and do **not** add
it to `Total`; that reasoning is also not priced (it is not treated as a
second output line).

## Per-model estimate

Since v0.6.0 the estimate is aggregated per **(vendor, normalized model)**:
each event's model id is normalized to the pricing card's canonical id,
priced against that card row, and summed per token category:

```text
model_cost = miss × miss_rate / 1M
           + cache_read × cache_read_rate / 1M
           + cache_create × cache_create_rate / 1M
           + output × output_rate / 1M
```

Unit prices are always displayed as **USD per 1M tokens**. A component with
no list rate stays unavailable (`—` / `null`), never `$0`; a card-listed
free component bills `$0` and is marked 限免. The dashboard 估价 cell shows
the two-decimal total and opens a per-vendor, per-model breakdown with a
TOTAL row; `wheretoken pricing --usage` prints the same rows in the
terminal; `/api/summary` exposes them as `by_model`.

Model ids that arrive **version-first** are normalized to the card's
family-first ids before matching: Cursor's API sends
`claude-4.6-opus-high-thinking` / `claude-4.5-sonnet` / `gpt-5-high`, which
price against the `opus-4.6` / `sonnet-4.5` / `gpt-5` rows. Trailing
effort/thinking suffixes (`-high`, `-low`, `-medium`, `-thinking`) are
stripped. The raw id stays on the event for drill-down display; only pricing
sees the normalized form, and resolution is memoized per
vendor/model/day so the aggregation cost does not scale with event count.

## Inspect the card

`wheretoken pricing` prints the same table the calculator reads — there is
no second copy to drift. Each vendor block shows the official source page
and the date a maintainer last verified the rates against it (a verification
date, not the program's run date).

```bash
wheretoken pricing                      # full card
wheretoken pricing --vendor anthropic   # one vendor
wheretoken pricing --model opus         # fuzzy / canonicalized match
wheretoken pricing --json               # stable schema for scripts
wheretoken pricing --usage              # your ledger, priced per model
wheretoken pricing --usage --since 7d   # report window flags apply
```

`--usage` runs a scan and prices each (vendor, model) against the same card:
per-category tokens (M), per-category unit rates (`—` when unlisted), model
cost, and a TOTAL row that matches the dashboard 估价. Unpriced models keep
their tokens visible with an unavailable cost, never a fake total. `--usage`
takes the report's window flags (`--today` / `--since` / `--from` / `--to`)
and `--offline`; `--usage --json` adds a `usage` block to the card payload.

An unlisted component renders as `—` (unknown, never `$0`); a card-listed
free component renders as `$0.00 限免`. In JSON those are `null` and
`0` + `"cache_create_free": true`. `--model` uses the calculator's own
normalization: `claude-opus-4-1` finds the `opus-4.1` row, while a dated
suffix like `claude-opus-4-1-20250805` intentionally matches nothing.

## Status

| `cost_status` | Meaning | Display |
| ------------- | ------- | ------- |
| `complete` | Every token matched a row on the open 2026-08-19 card (including older timestamps). Not a 2025 invoice, cache-TTL, or long-context tier | `cost_usd` |
| `partial` | Some tokens priced, some not | `cost_usd` of the priced part + `unpriced_tokens` |
| `unavailable` | No priced tokens | **omit** `cost_usd` — never `$0.00` |

An amount that rounds to `$0.0000` is omitted (not written as a zero bill).
Zero tokens with no priced work is `unavailable`. Missing price is
`unavailable`. Neither is `$0`.

## Card

`internal/price` ships card `2026-08-19` (public API list prices). Events use
the rate whose `[From, To)` contains the timestamp. Undated events use the
open-ended current card.

**Limitation:** this repository does not keep a full historical archive of
every vendor price change. Re-scanning last year's Claude usage with only the
2026-08-19 card applies that card, not the 2025 invoice. Retired Claude Opus 4
/ 4.1 rates on this card are the current list for those model ids, not what
they cost in 2025.

Anthropic cache writes use the **5-minute** list rate. The ledger does not say
whether a write was 5 minutes or 1 hour.

xAI uses the **short-context** tier. Long-context rates (≥200k prompt) are not
applied: the ledger does not store prompt length. `grok-4`, `grok-4-fast`, and
`grok-4-latest` stay unpriced: the public list has no grok-4 row. A card with
no cache-write rate does not treat cache-write tokens as `$0`; that event
stays unavailable.

OpenAI `o4-mini` uses the **standard** list, not Batch or Flex.

MiniMax **M2.1** / **M2.5** / **M2.7** (and their highspeed ids) use the
international pay-as-you-go list (not Token Plan credits). MiniMax **M3** stays
unpriced: the public page splits ≤512k / >512k and prints strikethrough promo
pairs.

Google Gemini **2.5 Flash / Lite**, **2.0 Flash / Lite**, and **3.5 Flash /
Lite** use the standard paid list (cache write is hourly storage, so
`CacheCreate>0` stays unpriced). Gemini **2.5 Pro** and other ≤200k / >200k
splits stay unpriced.

Qwen / DashScope coder models are context-tiered (`qwen3-coder-plus` steps
at 32k / 128k / 256k) and stay unpriced.

Moonshot **kimi-k3**, **kimi-k2.7-code** (+ highspeed), **kimi-k2.6**, and
**kimi-k2.5** use the official USD list (no cache-write token rate). Bare
`k3` stays unpriced.

Z.ai **glm-5.x** / **glm-4.7** (and 4.6 / 4.5) use the public USD list,
plus **glm-4.7-flashx**, **glm-4.5-x**, **glm-4.5-air**, and
**glm-4.5-airx**. Cache-write storage is listed as limited-time free, so
`CacheCreate` bills $0 there. List-free Flash rows stay unpriced rather
than rendering as $0.

DeepSeek **v4-flash** / **v4-pro** (and flash-vision-exp) use the public
USD list at **peak** rates. The off-peak half-price windows (01:00-04:00,
06:00-10:00 UTC Mon-Fri) are not applied; context caching has no write
charge, so `CacheCreate>0` stays unpriced.

ByteDance / Doubao (CNY / length bands) stays
unavailable. There is no Anthropic **Haiku 4** list row (only 4.5). xAI
coding list id is **`grok-build-0.1`**.

## Cursor reconciliation

Cursor's usage API exposes token columns per model but **no spend field**:
the amount Cursor actually billed is unavailable through the current API
surface. whereToken's Cursor estimate is therefore API-equivalent list cost
only — what the same tokens would cost on the public API card, not what the
subscription or plan charged.

The v0.6.0 normalization matters most here. v0.5.0 matched Cursor's
version-first ids against family-first card rows and priced almost nothing:
on the maintainer's own ~7.9B-token Cursor ledger the estimate moved from
**$6.90** (v0.5.0, ids unmatched) to **$864.00** (v0.6.0, model-level) on
the same ledger and the same card. A pinned synthetic fixture (3.6B tokens
of version-first ids) computes **$14,190.50** so this cannot silently
regress.

## What is not priced

Moonshot / Kimi Code, MiniMax-M3, Doubao, Alibaba, and any
unknown model stay `unavailable` until a verifiable public API list price is
added. Coding-agent subscriptions are not treated as API list prices.

## Sources

- Anthropic: https://platform.claude.com/docs/en/about-claude/pricing (verified 2026-08-19)
- xAI short-context tier: https://docs.x.ai/developers/pricing (verified 2026-08-19)
- OpenAI: https://developers.openai.com/api/docs/pricing (verified 2026-08-19)
- Moonshot: https://platform.kimi.ai/docs/pricing (verified 2026-08-20)
- MiniMax pay-as-you-go: https://platform.minimax.io/docs/guides/pricing-paygo (verified 2026-08-19)
- Z.ai: https://docs.z.ai/guides/overview/pricing (verified 2026-08-25)
- Google: https://ai.google.dev/gemini-api/docs/pricing (verified 2026-08-13)
- DeepSeek (peak rates): https://api-docs.deepseek.com/quick_start/pricing (verified 2026-08-25)

Official vendor pages only — no aggregators. A 2026-09-01 re-audit against
public snapshots of those pages found no conflicting official prices; the
few component rates that snapshots did not re-confirm (MiniMax cache write,
DeepSeek cache read) were kept unchanged rather than guessed. The CLI prints
the same metadata: `wheretoken pricing`.
