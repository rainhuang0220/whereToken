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
reasoning into `Output` (Codex, OpenCode), only `Output` is priced. Grok and
MiniMax store reasoning beside output and do **not** add it to `Total`; that
reasoning is also not priced (it is not treated as a second output line).

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

## What is not priced

Moonshot / Kimi Code, MiniMax-M3, Doubao, Alibaba, and any
unknown model stay `unavailable` until a verifiable public API list price is
added. Coding-agent subscriptions are not treated as API list prices.

## Sources

- Anthropic: https://platform.claude.com/docs/en/about-claude/pricing
- xAI short-context tier: https://docs.x.ai/developers/pricing
- OpenAI: https://developers.openai.com/api/docs/pricing
- MiniMax pay-as-you-go: https://platform.minimax.io/docs/guides/pricing-paygo
- Z.ai: https://docs.z.ai/guides/overview/pricing
- DeepSeek (peak rates): https://api-docs.deepseek.com/quick_start/pricing
