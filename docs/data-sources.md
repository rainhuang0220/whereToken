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
   metric.Aggregate (merge by Source+RequestID, max per field)
```

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

Do not read `settings.json` (may contain `ANTHROPIC_AUTH_TOKEN`) or `stats-cache.json`.

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

`~/.kimi-code/` and `~/.kimi/` (same inode is one root). Usage is `sessions/<workDirKey>/<sessionId>/agents/*/wire.jsonl`.

Skip `telemetry/`, `credentials/`, `config.toml`, `state.json`.

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

`~/.openclaw/agents/<agent>/sessions/<session>.jsonl`.

Do not read `*.trajectory.jsonl`, `skills-prompts/`, `credentials/`,
`identity/`, `openclaw.json`, or workspace trees. Never map `usage.cost`.

### Parser

`internal/adapter/openclaw`. Walks session JSONL. `type=message` with
`role=assistant` and `message.usage` becomes a usage event. `role=user` is a
user turn. `toolResult` rows are skipped even if they carry usage.
`type=session` supplies `cwd` and session id.

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

Trajectory files look like usage but also hold prompts. They are skipped.

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

Because totals are cumulative, Codex uses `index.LoadOrReplay` (no mid-file resume).

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

Login: `ItemTable` `cursorAuth/accessToken` (then `storage.json`). 401 may refresh in memory using `cursorAuth/refreshToken`. Never print the token. Hosts: `api2.cursor.sh` / `cursor.com`.

### Parser

Local bubbles: `type=2` (not thinking capability 30) → requests; `type=1` non-subagent → user turns.

Token columns: `POST` Cursor `DashboardService/GetFilteredUsageEvents` (fallback `GetAggregatedUsageEvents`), last 53 local weeks. When the API has totals, local `tokenCount` is ignored so the two are not added together. `--offline` skips the API.

### Token mapping

| whereToken | Source | Kind |
| ---------- | ------ | ---- |
| Miss / Cache / Output | API `inputTokens` / `cacheReadTokens` / `cacheWriteTokens` / `outputTokens` | provider_api |
| Requests / turns | local bubbles | raw |
| Local `tokenCount` | used only if the API has no token totals | raw |

Quality `authoritative` when the API (or local tokenCount) has numbers; `degraded` when the ledger exists but tokens are missing; `absent` when only an empty `~/.cursor` is found.

### Limitations

API token window is ~53 weeks; request/turn counts are all local sessions. Needs the user **signed in**. Encrypted storage is reported, not decrypted.

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

---

## Incremental index

JSONL adapters (Claude, Kimi, Grok, OpenClaw) cache normalized events by path / size / mtime / inode / offset. Appends parse only the new tail. Truncation or a new inode is a full rescan of that file.

Codex, OpenCode, and MiniMax Agent replay an unchanged file and fully reparse on any change.

`wheretoken rebuild` deletes `~/.cache/wheretoken/index.v1.db` (or `WHERETOKEN_INDEX`). The next scan reads agents again. The index is not a source of truth.

---

## Network

Most adapters never leave the machine. Cursor and Trae, when not `--offline` and a local login exists, call **that product’s** usage API. There is no whereToken cloud and no telemetry.
