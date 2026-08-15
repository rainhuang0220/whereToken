# whereToken

See where your **local** coding-agent tokens went. One command, a character table. No cloud, no USD prices, no telemetry.

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
wheretoken
```

Windows / macOS / Linux, without installing Go, after a [GitHub Release](https://github.com/rainhuang0220/whereToken/releases):

```bash
npm install -g wheretoken
# or: npx wheretoken
```

`go install` needs **Go 1.25+**. The npm wrapper downloads the release binary for your OS (or prints the `go install` line if that tag is not out yet).

## What you see

Six figures **since records began**, then rankings. Units are **M** (million tokens). Hit rate is cache-read on the input side only.

```
whereToken · 有账本以来

┌────────────┬────────────┬────────────┐
│ 总用量     │ 命中率     │ 最长连烧   │
│ 2299.98 M  │ 89.8%      │ 13 天      │
├────────────┼────────────┼────────────┤
│ 当前连烧   │ 请求       │ 用户回合   │
│ 7 天       │ 52,167     │ 2,216      │
└────────────┴────────────┴────────────┘
  合计 = 未命中 + 缓存读 + 缓存写 + 输出。命中率不含输出。

工具               合计   命中率     请求    回合
─────────────────────────────────────────────────
Cursor        1561.08 M    92.5%   45,737   1,861
Claude Code    358.23 M    70.3%    4,080     105
Kimi Code      330.04 M    98.8%    1,661      44
…
Trae             0.00 M        —        0       0
```

Your numbers will differ. Trae/Cursor token columns need that app signed in; the table says so instead of crashing or faking zeros as “unused.”

```bash
wheretoken --today              # local timezone; weeks start Monday
wheretoken --cursor             # also --claude --kimi --codex --opencode --trae
wheretoken --tool=claude
wheretoken --vendor=anthropic
wheretoken --model=k3
wheretoken --today --cursor
wheretoken --json               # scripts; tables stay the default
wheretoken --ascii              # old Windows consoles
wheretoken completion zsh          # also bash, fish, powershell
```

Checked-in scripts: [`completions/`](completions/).

**Tool ≠ vendor.** Claude Code running MiniMax still counts as tool Claude Code, vendor MiniMax.

Exit codes: `0` ok (including zero data or a degraded login), `1` runtime failure, `2` usage (unknown command / tool / vendor / model).

### Dashboard

GitHub Release binaries embed the kiln UI. `go install` embeds a short HTML stub; from a clone, `cd web && npm run build` then `wheretoken serve` uses `web/dist`.

```bash
go run ./cmd/wheretoken serve
```

Opens [http://127.0.0.1:8787](http://127.0.0.1:8787). **刷新** rescans disk (and signed-in Cursor/Trae). Reloading the tab does not.

## Privacy

Read-only local ledgers. HTTP binds `127.0.0.1` only. No telemetry. The CLI never prints JWTs or access tokens. Do not paste secrets into issues.

Paths come from the current user home (`os.UserHomeDir`, XDG, `~/Library/Application Support`, `%APPDATA%`). Missing dirs are skipped. Extra homes: `WHERETOKEN_EXTRA_ROOTS` (Unix `:`, Windows `;`, commas also work). Codex also reads `CODEX_HOME`. `WHERETOKEN_HOME` / `--home` fake a home for tests. `NO_COLOR` and `WHERETOKEN_ASCII=1` control the table.

## Data sources

Default scan only covers tools that already have a ledger on disk. macOS and Linux are first-class; Windows uses `%APPDATA%` the same way.

| Source | Typical path | Login for token columns? |
|--------|----------------|--------------------------|
| Claude Code | `~/.claude/projects/**/*.jsonl` | No |
| Kimi Code | `~/.kimi-code/` | No |
| Codex | `${CODEX_HOME:-~/.codex}/sessions/**/rollout-*.jsonl` | No |
| OpenCode | `$XDG_DATA_HOME/opencode` or `~/.local/share/opencode` | No |
| Cursor | macOS Application Support / Linux `~/.config/Cursor` / Windows `%APPDATA%\Cursor` | **Yes** for tokens; local bubbles still count requests/turns |
| Trae / Trae CN / TRAE SOLO | Application Support / `%APPDATA%\Trae*` | **Yes**; encrypted `storage.json` is reported, not decrypted |

Field mapping: [`docs/data-sources.md`](docs/data-sources.md). Not in this release: Windsurf, Copilot, Cline, Lingma, and similar, until there is a clear local token ledger.

`wheretoken scan --json` is the same payload as the dashboard **刷新**.

## Develop

```bash
go test ./...
make test                          # go + web + npm wrapper
bash scripts/verify-cli.sh         # table against testdata, not $HOME
bash scripts/verify-local.sh       # optional: this machine's ledgers
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

CI runs `go test` on Ubuntu, macOS, and Windows. Workflow files: [`ci/github-workflows/`](ci/github-workflows/) (`scripts/install-github-workflows.sh` copies them to `.github/workflows` when the remote token allows it).

## License

MIT. See [`LICENSE`](LICENSE).

---

## 中文

本机 coding agent 的 token 用量。装好后输入 `wheretoken` 得到字符表：有账本以来的总用量（M）、命中率、最长/当前连烧、请求、用户回合，再按工具和厂家排。`wheretoken serve` 仍是窑墙观察台（`127.0.0.1`）。不写美元价，不做遥测，不打印 JWT。工具 ≠ 厂家。
