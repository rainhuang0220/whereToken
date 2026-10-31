# Data Sources

whereToken reads usage that coding agents already store. Adapters discover those
files (and, for Cursor and Trae, that product’s own usage API), parse them, and
emit normalized events. This document is the adapter contract. Sample numbers
from a development machine live in [`data-sources/fixtures.md`](data-sources/fixtures.md).

## Overview

```text
Home (os.UserHomeDir / XDG / Application Support / %APPDATA%)
        ↓
   Adapter.Discover
        ↓
   Adapter.Parse   →  UsageEvent + TurnEvent
        ↓
   durable ledger (confirmed facts; not provider_api rows)
        ↓
   metric.Aggregate (merge by Source+RequestID, max per field)
```

The scan index (`index.v1.db`) is a cache of offsets and parsed blobs.
`wheretoken rebuild` deletes that cache. Confirmed facts live in the ledger
described in [`token-accounting.md`](token-accounting.md). Importing the cache
into the ledger takes a SQLite `VACUUM INTO` snapshot first. A blob that
cannot be decoded is counted as a gap and is not replaced with a zero.

Rules that apply to every adapter:

- Read-only. Extra homes: `WHERETOKEN_EXTRA_ROOTS`.
- Skip `auth.json`, `credentials/`, Keychain, Cookies, and tables whose columns are access tokens.
- Do not write prompt text into whereToken’s own files.
- Tool (coding agent) and vendor (model provider) are different axes. Claude Code serving `MiniMax-M3` is tool=Claude Code, vendor=MiniMax.
- Missing usage is `absent` / `degraded`, never a silent zero.
- Token field meanings: [`token-accounting.md`](token-accounting.md).

---

## Claude Code

### Location

`~/.claude/projects/<workspace-slug>/*.jsonl` (and `~/.config/claude/projects` as a fallback). Includes `*/subagents/*.jsonl`.

Do not read `settings.json` (may contain `ANTHROPIC_AUTH_TOKEN`),
`stats-cache.json`, or `~/.claude/feedback-bundles/` (full transcripts).

### Parser

`internal/adapter/claude`. Walks `*.jsonl`. `type=assistant` rows with `message.usage` become usage events. `type=user` content that is not a `tool_result` becomes a user turn.

Request id is `requestId`, else `message.id`. Per-line `uuid` is ignored.

Malformed JSON lines are skipped; later lines still parse.

### Token mapping

| whereToken | Claude field | Kind |
| ---------- | ------------ | ---- |
| Miss | `input_tokens` | raw |
| Cache Read | `cache_read_input_tokens` | raw |
| Cache Create | `cache_creation_input_tokens` | raw |
| Output | `output_tokens` | raw |

Duplicate `requestId` / `message.id` rows keep the **max of each field**. Quality is `degraded` because stream placeholders are common. Derivation is `deduplicated`.

### Quality

`degraded`. Cache columns are usually trustworthy; miss/output often stay at 0/1 until a later row.

### Limitations

A request that only ever wrote placeholders under-counts. Incremental scan reads appended JSONL bytes; `wheretoken rebuild` if a file was rewritten in place mid-line.

---

## Kimi Code

### Location

`~/.kimi-code/` and `~/.kimi/` (same inode is one root), or `KIMI_CODE_HOME`.
Usage is `sessions/<workDirKey>/<sessionId>/agents/*/wire.jsonl`.

Skip `telemetry/`, `credentials/`, `config.toml`, `state.json`.
`usageScope=session` rows are cumulative and are not summed. `/coding/v1/usages`
is a quota API (needs a key) and is not used.

### Parser

`type=usage.record` → usage. `type=turn.prompt` with `origin.kind=user` → user turn.

### Token mapping

| whereToken | Kimi field | Kind |
| ---------- | ---------- | ---- |
| Miss | `usage.inputOther` | raw |
| Cache Read | `usage.inputCacheRead` | raw |
| Cache Create | `usage.inputCacheCreation` | raw |
| Output | `usage.output` | raw |

Quality `authoritative`. Derivation `raw`. Timestamp is `time` (unix ms).

### Limitations

Only `wire.jsonl`. Same-millisecond records stay distinct requests.

---

## Grok CLI

### Location

`~/.grok/sessions/<url-encoded workspace>/<sessionId>/updates.jsonl`.

Do not read `auth.json`, `chat_history.jsonl`, `events.jsonl`, `summary.json`, `terminal/`, compaction trees. Never map `costUsdTicks`.

### Parser

`sessionUpdate=turn_completed` with `usage` and `prompt_id`. `modelUsage` with more than one model splits as `prompt_id:model`. `user_message_chunk` that is not a `<system-reminder>` is a user turn.

### Token mapping

| whereToken | Grok field | Kind |
| ---------- | ---------- | ---- |
| Miss | `inputTokens - cachedReadTokens - cacheCreationTokens` (min 0) | derived |
| Cache Read | `cachedReadTokens` | raw |
| Cache Create | `cacheCreationTokens` | raw |
| Output | `outputTokens` | raw |
| Reasoning | `reasoningTokens` | raw (not in Total) |

Quality `authoritative`. Derivation `derived`. Time prefers `_meta.agentTimestampMs`.

---

## MiniMax Agent

### Location

`~/.minimax/v2/sqlite/runtime-state.sqlite`.

Do not read `local-runtime.auth.json`, `sqlite.db` (agent process table),
`context-snapshots/`, `context-replacements/`, Application Support cookies,
or message bodies. Never map `cost_usd`.

### Parser

`internal/adapter/minimax`. Open SQLite read-only. Usage is
`local_runtime_token_usage`. Each row is one model request. Same `turn_id`
stays distinct (an agent turn can contain many requests). Request id is the
table `id`, not a per-line uuid.

User turns: `local_runtime_message_rows` where `role=user`. Workspace comes
from `local_runtime_sessions.record_json.workspaceDir` only.

The file is replay-cached by size/mtime/inode, not byte-offset.

### Token mapping

| whereToken | MiniMax field | Kind |
| ---------- | ------------- | ---- |
| Miss | `input_tokens` | raw |
| Cache Read | `cache_read_tokens` | raw |
| Cache Create | `cache_write_tokens` | raw |
| Output | `output_tokens` | raw |
| Reasoning | `reasoning_tokens` | raw (not in Total) |

Quality `authoritative`. Derivation `raw`. Timestamp is `ts` (unix ms).

Vendor comes from `model` via `vendor.Lookup` (MiniMax / DeepSeek / …), not
from the tool id.

### Limitations

No `token_usage` table (older install) is empty, not an error. Cache-read
values are per-request hits; summing them is the billed-equivalent cache
read, not a single context window.

---

## OpenClaw

### Location

`~/.openclaw/agents/<agent>/sessions/<session>.jsonl` plus the same
transcript after `/reset` or delete (`*.jsonl.reset.<ts>`,
`*.jsonl.deleted.<ts>`). Relocated homes: `OPENCLAW_STATE_DIR` /
`OPENCLAW_HOME`. Compressed delete archives (`*.jsonl.deleted.*.zst`)
are skipped, not parsed as JSONL. The live runtime after 2026.7.2 is
`~/.openclaw/agents/<agent>/agent/openclaw-agent.sqlite` plus
`state/openclaw.sqlite`. Those files mix session JSON with auth and
transcripts; whereToken does **not** open them.

Do not read `skills-prompts/`, `credentials/`, `identity/`,
`openclaw.json`, workspace trees, or the per-agent `agent/` directory.
Never map `usage.cost`. `*.trajectory.jsonl` holds prompts; it is used
only when that session has no transcript, and then only `data.usage`
numbers.

### Parser

`internal/adapter/openclaw`. Walks session JSONL (active and
`.reset` / `.deleted` archives). `type=message` with `role=assistant` and
`message.usage` becomes a usage event. `role=user` is a user turn.
`toolResult` rows are skipped even if they carry usage. `type=session`
supplies `cwd` and session id. Trajectory `type=model.completed` is a
fallback for sessions that have no transcript.

Request id is `message.responseId`. Per-line `id` is ignored.

Malformed JSON lines are skipped; later lines still parse.

### Token mapping

| whereToken | OpenClaw field | Kind |
| ---------- | -------------- | ---- |
| Miss | `message.usage.input` | raw |
| Cache Read | `message.usage.cacheRead` | raw |
| Cache Create | `message.usage.cacheWrite` | raw |
| Output | `message.usage.output` | raw |

Quality `authoritative`. Derivation `raw`. Timestamp is `message.timestamp`
(RFC3339).

Vendor comes from `message.model` / `message.provider` via `vendor.Lookup`.

### Limitations

A `/reset` or delete that only renames the JSONL must not drop historical
tokens: the archive name is still a transcript. Trajectory files also hold
`finalPromptText` / `messagesSnapshot`; those strings are never copied onto
events. Sibling trajectory + JSONL is not added together.

---

## OpenCode

### Location

`~/.local/share/opencode/opencode.db` or `opencode-stable.db` (XDG data). `~/.opencode` and `~/.config/opencode` are not ledgers.

Open SQLite read-only. Do not read `account` / `control_account` / `credential`.

### Parser

`message.data` JSON. Prefer message-level tokens (do not also sum `part.data` `step-finish`).

### Token mapping

| whereToken | OpenCode field | Kind |
| ---------- | -------------- | ---- |
| Miss | `tokens.input` | raw |
| Cache Read | `tokens.cache.read` | raw |
| Cache Create | `tokens.cache.write` | raw |
| Output | `tokens.output + tokens.reasoning` | derived |
| Reasoning | `tokens.reasoning` | raw (also in Output) |

Quality `authoritative`. Derivation `derived`. Rows without `time.created` stay undated.

The file is replay-cached by size/mtime/inode, not byte-offset.

---

## Codex

### Location

`${CODEX_HOME:-~/.codex}/sessions/YYYY/MM/DD/rollout-*.jsonl` and `archived_sessions/`.

Do not read `auth.json` or `logs_2.sqlite`.

### Parser

Stream line by line. Usage is `type=event_msg` / `payload.type=token_count`. Prefer advancing `info.total_token_usage`; otherwise `last_token_usage` with de-duplication. Model comes from the latest `turn_context`. User turns: `response_item` message with `role=user`.

Because totals are cumulative, Codex uses `index.LoadOrReplay` (no mid-file resume). A rollout this machine already parsed, which Codex later deletes, is replayed from that cache. `wheretoken rebuild` forgets it.

### Token mapping

| whereToken | Codex field | Kind |
| ---------- | ----------- | ---- |
| Miss | Δ `input_tokens` − Δ `cached_input_tokens` (min 0) | derived |
| Cache Read | Δ `cached_input_tokens` | derived |
| Cache Create | (none) | — |
| Output | Δ `output_tokens` + Δ `reasoning_output_tokens` | derived |
| Reasoning | Δ `reasoning_output_tokens` | derived (also in Output) |

Quality `authoritative`. Derivation `derived`.

---

## Cursor

### Location

`~/Library/Application Support/Cursor/User/globalStorage/state.vscdb` (Linux `~/.config/Cursor/…`, Windows `%APPDATA%\Cursor\…`). `~/.cursor` is a discover fallback only.

Login, only when the stored value is already a JWT: `ItemTable` `cursorAuth/accessToken` (then `storage.json`). A 401 may refresh in memory when `cursorAuth/refreshToken` is a different JWT. The same JWT is not posted to `/oauth/token`. Never print the token. Hosts: `api2.cursor.sh` / `cursor.com`.

A value that starts with `cursor-safe-storage:` (any version), or any other non-JWT, is not a bearer. whereToken does not read Keychain, DPAPI, or libsecret, and it does not send that value anywhere. Account usage is then unavailable. This is the same rule on macOS, Windows, and Linux.

### Parser

Local bubbles: `type=2` (not thinking capability 30) → requests; `type=1` non-subagent → user turns. A bubble with no `tokenCount` still counts as a request. Its token total is unavailable, not `0`. An explicit `tokenCount` of zero stays zero on the event and stays degraded.

Token columns, only for a readable JWT: `POST` Cursor `DashboardService/GetFilteredUsageEvents` (fallback `GetAggregatedUsageEvents`), last 53 local weeks. When the API has totals, local `tokenCount` is ignored so the two are not added together. `--offline` skips the API. A sealed session is still reported unavailable while offline, so a later refresh can keep a retained account observation instead of replacing it with local rows.

Filtered pagination is atomic. The response must include `totalUsageEventsCount`, and the raw row count must equal it. A short page, a missing total, a page that disagrees, a 429, or a schema change discards every row from that call. Aggregated usage is a fallback only when it returns events. An empty aggregated body after a filtered failure is not a measured zero. A complete filtered response with total `0` is a measured zero.

### What a normal install can read

| Situation | Account tokens | Local requests / turns | Extra setup |
| --------- | -------------- | ---------------------- | ----------- |
| macOS Cursor 3.x with encryption-at-rest (`cursor-safe-storage:` in `state.vscdb`; the key stays in the login item Cursor Safe Storage) | Unavailable | Yes | None. Do not edit the keychain ACL |
| Windows, DPAPI / ciphertext in the same ItemTable key | Unavailable | Yes | None. Do not call DPAPI |
| Linux, libsecret / ciphertext in the same ItemTable key | Unavailable | Yes | None. Do not call secret-tool |
| Any OS, older Cursor, plaintext JWT already in ItemTable or `storage.json` | Yes, private DashboardService, about 53 weeks | Yes | None. The API is not a public personal-usage contract and can change |
| No session row | Unavailable (not signed in) | Yes, when bubbles exist | None |
| User-exported dashboard CSV | Not imported | — | Not a zero-config source. The columns are not a verified stable contract |
| `api.cursor.com` Admin / Analytics, or a `crsr_` key | Not used for personal usage | — | Team or admin credentials. Not the default install |

DashboardService and the cursor.com dashboard cookie APIs are private. A personal API key cannot read this usage. whereToken does not ship a helper, a private certificate, or a keychain trust entry.

### Token mapping

| whereToken | Source | Kind |
| ---------- | ------ | ---- |
| Miss / Cache / Output | API `inputTokens` / `cacheReadTokens` / `cacheWriteTokens` / `outputTokens` | provider_api |
| Requests / turns | local bubbles | raw |
| Local `tokenCount` | used only if the API has no token totals | raw |

Quality `authoritative` when a complete API response has numbers, including a proven empty list; `degraded` when only local `tokenCount` exists; `absent` when only an empty `~/.cursor` is found. A degraded source whose token total is 0 is shown as unavailable, not `0.00 M`. Request counts stay.

Local requests and turns are activity counts. They become an account token total only when a complete account observation exists. Scan index mode `full` means that file was reread. A retained row keeps the earlier observation on its own window and stays separate from local `tokenCount`. A missing `tokenCount` stays unavailable. An explicit local zero stays zero and degraded.

### Limitations

The API window is about 53 weeks. Request and turn counts are local sessions on this machine. The account API, when it is readable, is one account window and must not be added across devices. A later successful scan of an append-only local log must not drop tokens. A failed or sealed account fetch does not delete a retained observation, does not add that observation on top of local `tokenCount`, and does not present the retained row as a fresh measurement. A narrower new window is still handled by the existing ledger supplement. Encrypted storage is reported, not decrypted.

---

## Trae

### Location

`~/Library/Application Support/{Trae,Trae CN,Trae-CN,TRAE SOLO,TRAE SOLO CN}/User/globalStorage/state.vscdb` (Linux `~/.config/Trae*`, Windows `%APPDATA%\Trae*`). JWT file `~/.trae-cn/trae-jwt-token` or `~/.trae/trae-jwt-token` when Trae writes it. Otherwise `storage.json` key `iCubeAuthInfo://icube.cloudide`: plaintext JSON, a raw JWT, or Trae's `tc` AES blob (same format Trae uses to restore login). The blob is decrypted only to take `token` and region; it is never printed.

Do not read Cookies, Keychain, `ModularData/ai-agent/database.db` (SQLCipher), skill trees, or prompt bodies.

### Parser

Session ids from memento keys only. Usage: `POST {host}/api/v1/commercial/get_session_usage` with `Cloud-IDE-JWT` and `X-User-Region` when known. CN host `trae-api-cn.mchost.guru`; international `coresg-normal.trae.ai`. Trae itself also calls this API and can get `empty_result` on credit accounts — that is empty usage, not a missing login.

### Token mapping

| whereToken | Trae field | Kind |
| ---------- | ---------- | ---- |
| Miss | `input_token - cache_read_token` (min 0) | derived |
| Cache Read | `cache_read_token` | raw |
| Cache Create | `cache_write_token` | raw |
| Output | `output_token` | raw |

Vendor comes from `model_name` via `vendor.Lookup` (DeepSeek / Doubao / …), not “trae”.

Quality `authoritative` when the API returns tokens; `degraded` when sessions exist but login is missing or still encrypted after decrypt.

`usage_time` on the API row is used as the event timestamp when present. Zero `usage_time` still has no date and is dropped by `--today` / `--since` / `--from` / `--to`.

### Limitations

Needs the user **signed in**. whereToken does not accept a pasted JWT.
Consumer Trae CN credit/free accounts often return `empty_result` from
the same session-usage API Trae itself calls. That is **unavailable**,
not measured zero. SQLCipher `ModularData/ai-agent/database.db` is not
read.

---

## Gemini CLI

### Location

`~/.gemini/tmp/<project>/chats/session-*.jsonl` (legacy `session-*.json`).
Windows: `%USERPROFILE%\.gemini\tmp\…`.

Do not read `oauth_creds.json`, `settings.json`, `google_accounts.json`,
or message `content`.

### Parser

`internal/adapter/gemini`. `type=gemini` rows with `tokens` become usage.
`type=user` is a user turn. Malformed lines are skipped.

### Token mapping

| whereToken | Gemini field | Kind |
| ---------- | ------------ | ---- |
| Miss | `tokens.input - tokens.cached` (min 0) | derived |
| Cache Read | `tokens.cached` | raw |
| Output | `tokens.output + tokens.thoughts` | derived (thinking billed as output) |
| Reasoning | `tokens.thoughts` | raw (also in Output; not added again) |

Quality `authoritative`. Official schema: `TokensSummary` in gemini-cli
`chatRecordingTypes.ts`.

---

## Qwen Code

### Location

`~/.qwen/usage/token-usage-*.jsonl` (Qwen Code `tokenUsageService`).

Do not read `settings.json`, `oauth_creds.json`, or chat transcripts under
`tmp/*/chats/` (those hold prompts). The usage ledger has no prompt fields.

### Token mapping

| whereToken | Qwen field | Kind |
| ---------- | ---------- | ---- |
| Miss | `inputTokens - cachedTokens` (min 0) | derived |
| Cache Read | `cachedTokens` | raw |
| Output | `outputTokens` | raw |
| Reasoning | `thoughtsTokens` | raw (not in Total) |

Quality `authoritative`. Incremental JSONL.

---

## Cline

### Location

VS Code-family `User/globalStorage/saoudrizwan.claude-dev/tasks/<id>/ui_messages.json`
(Code, VSCodium, Cursor, Windsurf, Insiders). Linux `~/.config/<product>/…`,
Windows `%APPDATA%\<product>\…`.

Do not read `settings/`, API keys, or conversation bodies except the
`api_req_started` / `deleted_api_reqs` / `subagent_usage` metrics JSON
that Cline itself sums in `getApiMetrics`.

### Token mapping

| whereToken | Cline field | Kind |
| ---------- | ----------- | ---- |
| Miss | `tokensIn` (uncached input) | raw |
| Cache Read | `cacheReads` | raw |
| Cache Create | `cacheWrites` | raw |
| Output | `tokensOut` | raw |

`cost` in those blobs is ignored. Quality `authoritative`.

---

## Roo Code

### Location

VS Code-family `User/globalStorage/RooVeterinaryInc.roo-cline/tasks/<id>/ui_messages.json`
(and `RooVeterinaryInc.roo-code-nightly`). Same product roots as Cline.

Do not read `settings/` or conversation bodies. Official
`consolidateTokenUsage` only sums `say=api_req_started` (not Cline's
`deleted_api_reqs` / `subagent_usage`).

### Token mapping

| whereToken | Roo field | Kind |
| ---------- | --------- | ---- |
| Miss | `tokensIn` | raw |
| Cache Read | `cacheReads` | raw |
| Cache Create | `cacheWrites` | raw |
| Output | `tokensOut` | raw |

`cost` ignored. Quality `authoritative`.

---

## Kilo Code (legacy VS Code)

### Location

VS Code-family `User/globalStorage/kilocode.kilo-code/tasks/<id>/ui_messages.json`
(also `kilocode.Kilo-Code`). Same product roots as Cline.

Do not read `settings/`, `api_conversation_history.json`, or conversation
bodies. Official leftover `consolidateTokenUsage` only sums
`say=api_req_started`.

The rebuilt **Kilo CLI 1.x** ledger is `~/.local/share/kilo/kilo.db`
(`KILO_DB` override). Same `message.data.tokens` shape as OpenCode.
OpenCode still only reads `opencode.db` — do not point it at `kilo/`.
Skip `auth.json` and credential tables.

### Token mapping

| whereToken | Kilo field | Kind |
| ---------- | ---------- | ---- |
| Miss | `tokensIn` | raw |
| Cache Read | `cacheReads` | raw |
| Cache Create | `cacheWrites` | raw |
| Output | `tokensOut` | raw |

`cost` ignored. Quality `authoritative`.

---

## ZCode (Z.ai ADE)

### Location

`~/.zcode/cli/db/db.sqlite` (`ZCODE_DB` override), table `model_usage` — one
row per request. Workspace labels join the `session` table. Sign-in state and
settings are never read.

Legacy `~/.zcode/projects/*.jsonl` transcripts are not read: lines without a
usage block would need token estimates, and whereToken does not estimate.

### Token mapping

| whereToken | ZCode field | Kind |
| ---------- | ----------- | ---- |
| Miss | `input_tokens − cache_read − cache_creation` | derived |
| Cache Read | `cache_read_input_tokens` | raw |
| Cache Create | `cache_creation_input_tokens` | raw |
| Output | `output_tokens − reasoning_tokens` | derived |
| Reasoning | `reasoning_tokens` | raw |

ZCode's `input_tokens` absorbs both cache buckets and `output_tokens` absorbs
reasoning. Newer stores carry `computed_total_tokens`: when it equals
`input + output` the row is inclusive and the overlap comes out; when it
equals the fully additive sum the row passes through. Older stores without
that column are always inclusive. Quality `authoritative`.

---

## Incremental index

JSONL adapters (Claude, Kimi, Grok, OpenClaw) cache normalized events by path / size / mtime / inode / offset. Appends parse only the new tail. Truncation or a new inode is a full rescan of that file.

Codex, OpenCode, and MiniMax Agent replay an unchanged file and fully reparse on any change.

`wheretoken rebuild` deletes `~/.cache/wheretoken/index.v1.db` (or `WHERETOKEN_INDEX`). The next scan reads agents again. The index is not a second ledger, except that a deleted Codex rollout stays replayable until that rebuild.

---

## Network

Most adapters never leave the machine. Cursor and Trae, when not `--offline` and a local login exists, call **that product’s** usage API. Community Rank is a separate, optional anonymous-aggregate layer (`docs/community.md`); it is not a dump of this ledger.
