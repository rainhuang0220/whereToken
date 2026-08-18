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
reasoning into `Output`, only `Output` is priced.

## Status

| `cost_status` | Meaning | Display |
| ------------- | ------- | ------- |
| `complete` | Every token in the slice matched a list price | `cost_usd` |
| `partial` | Some tokens priced, some not | `cost_usd` of the priced part + `unpriced_tokens` |
| `unavailable` | No priced tokens | **omit** `cost_usd` — never `$0.00` |

Zero tokens on a known model is a real zero (`complete`, `$0.0000`).
Missing price is `unavailable`.

## Card

`internal/price` ships card `2026-08-19` (public API list prices). Events use
the rate whose `[From, To)` contains the timestamp. Undated events use the
open-ended current card.

**Limitation:** this repository does not keep a full historical archive of
every vendor price change. Re-scanning last year's Claude usage with only the
2026-08-19 card applies that card, not the 2025 invoice.

## What is not priced

Moonshot / Kimi Code, MiniMax, DeepSeek, Doubao, Zhipu, Alibaba, and any
unknown model stay `unavailable` until a verifiable public API list price is
added. Coding-agent subscriptions are not treated as API list prices.

## Sources

- Anthropic: https://platform.claude.com/docs/en/about-claude/pricing
- xAI short-context tier: https://docs.x.ai/developers/pricing
- OpenAI: https://developers.openai.com/api/docs/pricing
