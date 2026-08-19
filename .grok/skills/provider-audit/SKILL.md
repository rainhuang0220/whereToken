---
name: provider-audit
description: Attack a whereToken adapter after implementation. Use when reviewing Gemini, Qwen, Cline, Roo, or any new collector for double count, secrets, or zero-vs-unavailable.
---

# Provider audit

After a collector ships, try to break it. Do not treat green tests as the end.

## Attacks

- Config dir without a ledger → usage must be `unavailable`, not `0`
- Neighbor `auth.json` / `oauth_creds.json` / settings API keys must not appear on events or in errors
- Malformed / truncated last JSONL line: do not consume; do not drop earlier rows
- Huge complete line: skip that line, keep siblings
- Duplicate `Source+RequestID`: max per field, not sum
- Negative token fields: drop the event
- Reasoning must not enter Total; Grok/MiniMax reasoning is not a second output price
- Window must not mark historical sources absent
- Incremental: append-only second scan ≥ first; parse error keeps cached blobs
- Windows `%APPDATA%` and Linux XDG paths via `testhome`

## Output

File:line of each hole, fixture if missing, class A–E confirmation in `docs/provider-matrix.md`.
