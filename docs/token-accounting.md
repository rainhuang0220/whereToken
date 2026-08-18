# Token accounting

whereToken normalizes each adapter into the same six figures. This file is the
product definition. The math follows the code in `internal/metric` and the
adapters; it is not adjusted to make the document prettier.

## Normalized fields

| Field | Meaning |
| ----- | ------- |
| **Miss** | Tokens billed as a fresh input read: prompt text the model had to ingest that was not served from cache. |
| **Cache Read** | Input tokens served from a prompt cache (Anthropic-style cache hits, Grok cached reads, Codex cached input, …). |
| **Cache Create** | Input tokens written into a prompt cache on this request (cache writes / cache creation). |
| **Output** | Completion tokens produced by the model. Some adapters fold reasoning tokens into this column; see the mapping table. |
| **Reasoning** | Hidden / reasoning tokens when the source exposes them as a separate number. Stored on the event for explanation. **Not added again** in `Total`. |
| **Total** | `Miss + Cache Read + Cache Create + Output`. |

```text
Total =
    Miss
  + Cache Read
  + Cache Create
  + Output
```

Reasoning is **not** a sixth addend. If an adapter already included reasoning
inside `Output` (Codex, OpenCode), adding `Reasoning` again would double-count.

Cache hit rate (when defined):

```text
hit_rate = Cache Read / (Miss + Cache Read + Cache Create)
```

Output is excluded from the hit-rate denominator.

## Quality

Quality is independent of the arithmetic. It answers “how much should you trust
this number?”, not “how was it computed?”.

| Quality | Meaning |
| ------- | ------- |
| `authoritative` | The source exposed these token fields directly and they are treated as complete. |
| `degraded` | A source exists but the numbers are incomplete (Claude stream placeholders after max-merge; Cursor/Trae without a usable login). |
| `estimated` | whereToken invented a figure. No adapter currently emits this. |
| `absent` | The agent was found (or expected) but there is no usage to show. **Not displayed as zero.** |

Missing usage is never rewritten as `0`.

## Derivation

Derivation answers “how did this value get onto the event?”.

| Derivation | Meaning |
| ---------- | ------- |
| `raw` | Copied from a source field with that meaning. |
| `provider_api` | Copied from that product’s own usage API. |
| `derived` | Computed from other source fields (subtract cache from inclusive input, convert cumulative totals to deltas, fold reasoning into output). |
| `deduplicated` | Several source rows share a request id; whereToken keeps the max of each field. |
| `estimated` | Reserved. Not used. |

## Provider → normalized field map

`Quality` in this table is the usual quality of a successful parse, not a
guarantee for every row.

| Provider | Raw field | whereToken field | Kind | Quality | Notes |
| -------- | --------- | ---------------- | ---- | ------- | ----- |
| Claude Code | `message.usage.input_tokens` | Miss | raw | degraded | Stream rows often start at 0; merge takes max per `requestId` / `message.id`. |
| Claude Code | `message.usage.cache_read_input_tokens` | Cache Read | raw | degraded | More reliable than miss/output on stream rows. |
| Claude Code | `message.usage.cache_creation_input_tokens` | Cache Create | raw | degraded | |
| Claude Code | `message.usage.output_tokens` | Output | raw | degraded | Known to stay at 0/1 on some stream placeholders. |
| Claude Code | `requestId` or `message.id` | Request id | raw | — | Per-line `uuid` is **not** a request id. |
| Kimi Code | `usage.inputOther` | Miss | raw | authoritative | `usage.record` only. |
| Kimi Code | `usage.inputCacheRead` | Cache Read | raw | authoritative | |
| Kimi Code | `usage.inputCacheCreation` | Cache Create | raw | authoritative | |
| Kimi Code | `usage.output` | Output | raw | authoritative | |
| Grok CLI | `inputTokens - cachedReadTokens - cacheCreationTokens` | Miss | derived | authoritative | `inputTokens` is inclusive of cache. Negative clamped to 0. |
| Grok CLI | `cachedReadTokens` | Cache Read | raw | authoritative | |
| Grok CLI | `cacheCreationTokens` | Cache Create | raw | authoritative | |
| Grok CLI | `outputTokens` | Output | raw | authoritative | |
| Grok CLI | `reasoningTokens` | Reasoning | raw | authoritative | Not added into Total. |
| MiniMax Agent | `input_tokens` | Miss | raw | authoritative | `local_runtime_token_usage` only. Each row is one request; same `turn_id` stays distinct. |
| MiniMax Agent | `cache_read_tokens` | Cache Read | raw | authoritative | Per-request cache hit, not a running total. |
| MiniMax Agent | `cache_write_tokens` | Cache Create | raw | authoritative | |
| MiniMax Agent | `output_tokens` | Output | raw | authoritative | Reasoning is not folded in. |
| MiniMax Agent | `reasoning_tokens` | Reasoning | raw | authoritative | Not added into Total. |
| Codex | Δ `input_tokens` − Δ `cached_input_tokens` | Miss | derived | authoritative | Deltas of `total_token_usage` (or `last_token_usage` if no running total). |
| Codex | Δ `cached_input_tokens` | Cache Read | derived | authoritative | |
| Codex | — | Cache Create | — | — | Not exposed. |
| Codex | Δ `output_tokens` + Δ `reasoning_output_tokens` | Output | derived | authoritative | Reasoning is folded into Output. |
| Codex | Δ `reasoning_output_tokens` | Reasoning | derived | authoritative | Also included in Output; not added again. |
| OpenCode | `tokens.input` | Miss | raw | authoritative | From `message.data`. |
| OpenCode | `tokens.cache.read` | Cache Read | raw | authoritative | |
| OpenCode | `tokens.cache.write` | Cache Create | raw | authoritative | |
| OpenCode | `tokens.output + tokens.reasoning` | Output | derived | authoritative | |
| OpenCode | `tokens.reasoning` | Reasoning | raw | authoritative | Also included in Output. |
| Cursor | API `inputTokens` | Miss | provider_api | authoritative | DashboardService. Local bubbles supply requests/turns, not this column, when the API has totals. |
| Cursor | API `cacheReadTokens` | Cache Read | provider_api | authoritative | |
| Cursor | API `cacheWriteTokens` | Cache Create | provider_api | authoritative | |
| Cursor | API `outputTokens` | Output | provider_api | authoritative | |
| Cursor | local `tokenCount.*` | Miss/Output | raw | degraded | Used only when the account API has no token totals. |
| Trae | `input_token - cache_read_token` | Miss | derived | authoritative | `input_token` includes cache read. |
| Trae | `cache_read_token` | Cache Read | raw | authoritative | |
| Trae | `cache_write_token` | Cache Create | raw | authoritative | |
| Trae | `output_token` | Output | raw | authoritative | |

## Request merge

`metric.Aggregate` merges events that share `Source + RequestID` by taking the
**maximum of each token field independently**. A row that only has output cannot
overwrite a sibling row’s miss.

Adapters must not invent a request id from a per-line uuid. That would defeat
the merge and double-count stream placeholders.

## Local scan index

The SQLite file under `~/.cache/wheretoken/` is a performance cache. File
identity is path + size + mtime + inode, not a content hash. `wheretoken rebuild`
deletes it. Incremental JSONL stores the last **consumed** byte offset, which
stays behind EOF while the last line is still being written.

## Cost

Token totals never become USD by a single global price. See [`cost.md`](cost.md).
Estimated API-equivalent cost is optional, never a bill, and never `$0` when
the price is unknown.

## What Total is not

Total is not USD, not a provider invoice, and not “context window used”.
Optional API-equivalent USD is a separate field (`docs/cost.md`), never folded
into Total, and never written as `$0` when the price is unknown.
