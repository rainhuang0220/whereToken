---
name: provider-research
description: Research whether a global AI coding tool has a safe local usage ledger for whereToken. Use when adding or evaluating Gemini, Qwen, Cline, Copilot, GLM, Doubao, Aider, or any new provider.
---

# Provider research

Do not stop because the maintainer’s machine lacks the tool. Read official docs, GitHub source, issues, and release notes.

## Required output

One matrix row for `docs/provider-matrix.md`: class A–E, local path, fields, safe?, API key?, cloud?, login?, incremental?

## Search like this

```
tool session jsonl usage tokens
tool token-usage local file
tool cacheRead cache_read tokens
~/.tool tmp chats usage
```

## Never

- Treat config (`settings.json`, `oauth_creds.json`) as usage.
- Recommend SQLCipher / Keychain / mixed auth SQLite.
- Invent prices or token fields.

A is implementable this week only if a ledger exists without prompts/secrets, or usage fields can be decoded without storing bodies (same as Claude JSONL).
