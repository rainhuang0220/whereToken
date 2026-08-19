# Provider capability matrix

whereToken is a **local-first** usage engine for coding agents worldwide.
A tool is listed here whether or not it is installed on the maintainer’s
machine. Classes:

| Class | Meaning |
| ----- | ------- |
| **A** | Reliable, local, safe usage ledger. Collector shipped or ready. |
| **B** | Tool can be detected; usage is unavailable (not zero). |
| **C** | Usage only via that product’s cloud API (local login, no pasted keys). |
| **D** | Possible but unsafe (credentials mixed with transcripts, encryption). |
| **E** | No reliable source found. |

Finding `~/.tool` is **discovery**, not usage.

| Tool | Class | Local source | Safe? | API key? | Cloud? | Login? | Incremental | Notes |
| ---- | ----- | ------------ | ----- | -------- | ------ | ------ | ----------- | ----- |
| Claude Code | A | `~/.claude/projects/**/*.jsonl` | yes | no | no | no | yes | Skip settings, feedback-bundles |
| Kimi Code | A | `~/.kimi-code/**/wire.jsonl` | yes | no | no | no | yes | Skip credentials/ |
| Grok CLI | A | `~/.grok/sessions/**/updates.jsonl` | yes | no | no | no | yes | Skip auth.json |
| MiniMax Agent | A | `runtime-state.sqlite` token table | yes | no | no | no | replay | Skip auth json |
| OpenClaw | A | session JSONL + reset/deleted | yes | no | no | no | yes | Never agent/*.sqlite |
| OpenCode | A | `opencode.db` message tokens | yes | no | no | no | replay | Skip account tables |
| Codex | A | `~/.codex/sessions/**/rollout-*.jsonl` | yes | no | no | no | replay | Skip auth.json |
| Cursor | C | product DashboardService | yes* | no | yes | yes | no | *local login only; 53-week window |
| Trae | B/C | session usage API | yes* | no | yes | yes | no | CN credits → empty_result = unavailable |
| Gemini CLI | A | `~/.gemini/tmp/*/chats/session-*` | yes | no | no | no | yes | Official `TokensSummary`; skip oauth |
| Qwen Code | A | `~/.qwen/usage/token-usage-*.jsonl` | yes | no | no | no | yes | Dedicated ledger; no prompts |
| Cline | A | `saoudrizwan.claude-dev/tasks/*/ui_messages.json` | yes | no | no | no | replay | Metrics JSON only; skip settings |
| Roo Code | A | `RooVeterinaryInc.roo-cline/tasks/*/ui_messages.json` | yes | no | no | no | replay | Official `api_req_started` only |
| GLM / Z.ai | E | none first-party | — | often | — | — | — | Vendor via host tools |
| Doubao / Volcengine | E | none first-party | — | yes for Ark | — | — | — | May appear as Trae model |
| DeepSeek | E | none first-party CLI ledger | — | balance API | — | — | — | Vendor via host tools |
| GitHub Copilot | B | `~/.copilot` config / no usage files | — | — | — | — | — | Schema has `assistant.usage`; no ledger on disk |
| Windsurf | E | none found | — | — | — | — | — | |
| Continue | B | `~/.continue` is config | — | — | — | — | — | Config ≠ usage |
| Roo Code | D | leftover Cline-shaped trees | mixed | — | — | — | — | Product shut down; transcripts |
| Aider | B | `.aider.chat.history.md` is prompts | no for bodies | — | — | — | — | `/tokens` is context window |
| Goose | E | research: no billed ledger | — | — | — | — | — | |
| OpenHands | E | research: no billed ledger | — | — | — | — | — | |
| SWE-agent | E | research: no billed ledger | — | — | — | — | — | |
| Kilo Code | E | OpenCode-fork CLI + rebuilt VS Code ext | — | — | — | — | — | Do not scrape Cline leftovers; would double-count OpenCode |
| ChatGPT desktop | E | no local request ledger | — | admin usage API | yes | — | — | Class C if org admin key (not used) |

Sources: official GitHub (gemini-cli `chatRecordingTypes.ts`, qwen-code `tokenUsageService.ts`, cline `getApiMetrics.ts`), plus existing whereToken adapters. Re-check when those products change storage.

`adapter.Catalog` is the machine-readable subset of **shipped** tools. This file is the global research table.
