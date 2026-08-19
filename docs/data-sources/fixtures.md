# Fixture notes

Development-machine snapshots used to size fixtures and check parsers. They are
**not** product claims.

Recorded 2026-08-15 on one macOS host.

## Claude Code

5 project directories, 41 JSONL files.

Raw (not request-deduped) order of magnitude: miss ~97.8M, cache_create ~9.0M,
cache_read ~252.1M, output ~1.3M. Assistant rows 4118; true user turns ~105
after dropping `tool_result`.

`testdata/adapters/claude/` is a desensitized slice. `claude_dup` and
`claude_malformed` cover merge and skip-bad-line.

## Kimi Code

12 `wire.jsonl` files, 1661 `usage.record` rows. miss ~3.9M, cache_read ~325.2M,
output ~0.9M, user turns 44.

`testdata/adapters/kimi/` is the golden mapping fixture.

## OpenCode

7 sessions. miss ~0.19M, cache_read ~1.53M, output ~0.02M. Session totals
matched message totals on that host.

## Codex

29 rollout files; largest ~24 MB. Must stream; do not slurp.

## Cursor

Local bubbles on that host: 1,827 user turns, 43,550 requests, `tokenCount`
almost all zero. Token columns come from the account API, not those zeros.

## OpenClaw

On one host (2026-08-20), 10 active session JSONL files held 67 usage
rows (~1.85M tokens). The same tree also had `/reset` and `/delete`
archives plus trajectory-only sessions; counting those archives is
required or a later `/reset` looks like a token drop.

`testdata/adapters/openclaw/` covers an active transcript, a `.reset`
archive, a `.deleted` archive, and a trajectory-only fallback.

## Grok / Trae

See `testdata/adapters/grok` and `internal/adapter/trae` tests. JWT fixtures
are the literal `test-token`.
