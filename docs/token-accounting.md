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
inside `Output` (Codex, OpenCode, Kilo CLI, ZCode), adding `Reasoning` again
would double-count.

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
| OpenClaw | `message.usage.input` | Miss | raw | authoritative | Session JSONL, including `.jsonl.reset.*` / `.jsonl.deleted.*`. |
| OpenClaw | `message.usage.cacheRead` | Cache Read | raw | authoritative | |
| OpenClaw | `message.usage.cacheWrite` | Cache Create | raw | authoritative | |
| OpenClaw | `message.usage.output` | Output | raw | authoritative | `usage.cost` is ignored. |
| OpenClaw | `data.usage.input` (trajectory) | Miss | raw | authoritative | Only when that session has no transcript. Prompts are discarded. |
| OpenClaw | `message.responseId` | Request id | raw | — | Per-line `id` is **not** a request id. Trajectory uses `runId`. |
| Codex | Δ `input_tokens` − Δ `cached_input_tokens` | Miss | derived | authoritative | Deltas of `total_token_usage` (or `last_token_usage` if no running total). |
| Codex | Δ `cached_input_tokens` | Cache Read | derived | authoritative | |
| Codex | Δ `cache_write_input_tokens` | Cache Create | derived | authoritative | CLI 0.145+; often 0 on older logs. |
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
| Gemini CLI | `tokens.input - tokens.cached` | Miss | derived | authoritative | Session `type=gemini`. |
| Gemini CLI | `tokens.cached` | Cache Read | raw | authoritative | |
| Gemini CLI | `tokens.output + tokens.thoughts` | Output | derived | authoritative | Thinking is billed as output (Codex fold). |
| Gemini CLI | `tokens.thoughts` | Reasoning | raw | authoritative | Also in Output; not added again. |
| Qwen Code | `inputTokens - cachedTokens` | Miss | derived | authoritative | `usage/token-usage-*.jsonl` only. |
| Qwen Code | `cachedTokens` | Cache Read | raw | authoritative | |
| Qwen Code | `outputTokens` | Output | raw | authoritative | |
| Qwen Code | `thoughtsTokens` | Reasoning | raw | authoritative | Not added into Total. |
| Cline | `tokensIn` | Miss | raw | authoritative | `ui_messages.json` metrics only. |
| Cline | `cacheReads` | Cache Read | raw | authoritative | |
| Cline | `cacheWrites` | Cache Create | raw | authoritative | |
| Cline | `tokensOut` | Output | raw | authoritative | `cost` ignored. |
| Roo Code | `tokensIn` | Miss | raw | authoritative | `api_req_started` only. |
| Roo Code | `cacheReads` | Cache Read | raw | authoritative | |
| Roo Code | `cacheWrites` | Cache Create | raw | authoritative | |
| Roo Code | `tokensOut` | Output | raw | authoritative | `cost` ignored. |
| Kilo Code | `tokensIn` | Miss | raw | authoritative | Legacy `api_req_started` only. |
| Kilo Code | `cacheReads` | Cache Read | raw | authoritative | |
| Kilo Code | `cacheWrites` | Cache Create | raw | authoritative | |
| Kilo Code | `tokensOut` | Output | raw | authoritative | `cost` ignored. |
| Kilo CLI | `tokens.input` | Miss | raw | authoritative | `kilo.db` `message.data`, same as OpenCode. |
| Kilo CLI | `tokens.cache.read` | Cache Read | raw | authoritative | |
| Kilo CLI | `tokens.cache.write` | Cache Create | raw | authoritative | |
| Kilo CLI | `tokens.output + tokens.reasoning` | Output | derived | authoritative | |
| Kilo CLI | `tokens.reasoning` | Reasoning | raw | authoritative | Also in Output. |
| ZCode | `input_tokens − cache_read − cache_creation` | Miss | derived | authoritative | `model_usage` input absorbs both cache buckets. |
| ZCode | `cache_read_input_tokens` | Cache Read | raw | authoritative | |
| ZCode | `cache_creation_input_tokens` | Cache Create | raw | authoritative | |
| ZCode | `output_tokens − reasoning_tokens` | Output | derived | authoritative | Output absorbs reasoning. |
| ZCode | `reasoning_tokens` | Reasoning | raw | authoritative | Also removed from Output; never in Total. |

## Request merge

`metric.Aggregate` merges events that share `Source + RequestID` by taking the
**maximum of each token field independently**. A row that only has output cannot
overwrite a sibling row’s miss.

Adapters must not invent a request id from a per-line uuid. That would defeat
the merge and double-count stream placeholders.

## What “总用量” is

With no window flags, CLI / `--json` / dashboard 全部 is **this scan’s
currently visible local data**: every adapter event from files still on
disk (and Cursor/Trae’s product API window when online). It is not “bytes
in `index.v1.db`” and not a provider invoice.

`--today` / `--since` / `--from` / `--to` filter that set in the local
timezone. A source that has history but nothing in the window is omitted
or shows 0 for the window; it is not marked “数据不可用”.

Missing usage (detected tool, no ledger) is **unavailable**, never `0.00 M`.

If source files only append, a later successful scan must not drop tokens.
A `/reset` that renames `session.jsonl` to `session.jsonl.reset.<ts>` is
still visible data. Truncation, deletion of the archive, a rolling cloud
window (Cursor ~53 weeks), or Trae `empty_result` can lower the number
and must be explainable.

## Local scan index

The SQLite file under `~/.cache/wheretoken/` is a performance cache. File
identity is path + size + mtime + inode, not a content hash. `wheretoken rebuild`
deletes it. Incremental JSONL stores the last **consumed** byte offset, which
stays behind EOF while the last line is still being written. A parse error
on the new tail keeps the cached events for that file.

## Cost

Token totals never become USD by a single global price. See [`cost.md`](cost.md).
Estimated API-equivalent cost is optional, never a bill, and never `$0` when
the price is unknown.

## User portrait

The dashboard's 用户画像 cell (`internal/profile`) summarizes the selected
window as a short Chinese phrase plus up to two tags. It is a deterministic
function of the summary metrics — no LLM, no embeddings, no network.

Eight dimensions of the window are each bucketed low / mid / high against
documented thresholds: intensity (tokens per active day), API-equivalent cost
per active day, model diversity, vendor diversity, cache reuse (hit rate),
consistency (active days over the window span), burstiness (peak day over
mean day), and concentration (top model's share of tokens). A fixed priority
chain picks the primary trait and up to two supporting traits; unmeasurable
dimensions (unpriced cost, no cache traffic, no labeled model) are skipped,
never treated as zero.

Wording comes from a ~200-phrase Chinese bank grouped by trait. The pick is
seeded by a local anonymous install identity — the community participant id
when present, otherwise a random UUIDv4 in
`~/.config/wheretoken/install-id` (0600) created on first use. The id is a
random UUID, contains no PII, and never leaves the device; username, HOME,
hostname, and IP are never read into the seed. Selection is bucket-based:
same data + same seed gives the same portrait, and small metric fluctuations
inside a bucket never change the wording — only crossing a bucket boundary
can.

With no records in the window the cell is `—`; below 100k window tokens it
is `数据不足`. Neither state invents a phrase.

## What Total is not

Total is not USD, not a provider invoice, and not “context window used”.
Optional API-equivalent USD is a separate field (`docs/cost.md`), never folded
into Total, and never written as `$0` when the price is unknown.
