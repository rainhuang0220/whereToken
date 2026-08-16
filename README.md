# whereToken

See where your **local** coding-agent tokens went. One command, a character table. No cloud, no USD prices, no telemetry.

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

Then run `wheretoken`. Archives are SHA-256 checked against `checksums.txt`. If you already have Go 1.25+:

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

From a clone, Homebrew can still build from source (needs Go):

```bash
brew install --HEAD ./Formula/wheretoken.rb
```

The `npm/` wrapper is **not on the npm registry** yet.

## Not yet

Short list, not a roadmap:

- GitHub Release binaries are **unsigned** until Apple signing secrets are set ([`docs/macos-signing.md`](docs/macos-signing.md)).
- There is **no npm package**.
- Trae and Cursor **token columns** need those apps **signed in** on this machine. Local Claude / Kimi / Codex / OpenCode ledgers do not.

## What you see

Six figures **since records began**, then rankings. Units are **M** (million tokens). Hit rate is cache-read on the input side only.

```
whereToken · 有账本以来
近7日  ▁▃▂▁█▅▃

┌────────────┬────────────┬────────────┐
│ 总用量     │ 命中率     │ 最长连烧   │
│ 2,323.43 M │      89.9% │      13 天 │
├────────────┼────────────┼────────────┤
│ 当前连烧   │ 请求       │ 用户回合   │
│       7 天 │     52,927 │      2,216 │
└────────────┴────────────┴────────────┘
  合计 = 未命中 + 缓存读 + 缓存写 + 输出。命中率不含输出。

工具                合计    占比   命中率     请求    回合
──────────────────────────────────────────────────────────
Cursor        1,584.54 M   68.2%    92.6%   46,497   1,861
Claude Code     358.23 M   15.4%    70.3%    4,080     105
Kimi Code       330.04 M   14.2%    98.8%    1,661      44
…
Trae              0.00 M    0.0%        —        0       0
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
wheretoken --width 40           # ranking drops 回合/请求 before turning names into C...
wheretoken --quiet              # no “正在读 …” on stderr
wheretoken --offline            # local ledgers only; skip Cursor/Trae APIs
wheretoken completion zsh          # also bash, fish, powershell
```

Checked-in scripts: [`completions/`](completions/). Manual page: [`docs/wheretoken.1`](docs/wheretoken.1).

```bash
wheretoken completion zsh > ~/.zfunc/_wheretoken   # then add ~/.zfunc to fpath
```

**Tool ≠ vendor.** Claude Code running MiniMax still counts as tool Claude Code, vendor MiniMax.

`--json` is **schema 1** ([`docs/cli-json.schema.json`](docs/cli-json.schema.json)): `period`, `total`, `total_m`, `hit_rate`, `requests`, `user_turns`, `tools`, `vendors`, `notes`. Each tool/vendor row includes raw `total` plus `total_m`. `--today` adds `models` and omits `last_7d` / streaks. `--model` sets `hide_turns: true` because turns are per-tool.

Exit codes: `0` ok (including zero data or a degraded login), `1` runtime failure, `2` usage (unknown command / tool / vendor / model).

### Dashboard

GitHub Release binaries embed the kiln UI. `go install` embeds a short HTML stub; from a clone, `cd web && npm run build` then `wheretoken serve` uses `web/dist`.

```bash
go run ./cmd/wheretoken serve
```

Opens [http://127.0.0.1:8787](http://127.0.0.1:8787). **刷新** rescans disk (and signed-in Cursor/Trae). Reloading the tab does not.

## Privacy

Read-only local ledgers. HTTP binds `127.0.0.1` only. No telemetry. The dashboard does not load third-party fonts. The CLI never prints JWTs, access tokens, API keys, or cookies. Do not paste secrets into issues. See [`SECURITY.md`](SECURITY.md).

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
make ci                            # fmt-check + vet + test + race + fixture CLI
bash scripts/verify-cli.sh         # table against testdata, not $HOME
bash scripts/verify-local.sh       # optional: this machine's ledgers
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

`go test` / `go install` use the **Go 1.25.13** toolchain from `go.mod` (Go 1.25.0+ will download it).

CI runs on GitHub Actions (Ubuntu, macOS, Windows). The same YAML is kept in [`ci/github-workflows/`](ci/github-workflows/).

## License

MIT. See [`LICENSE`](LICENSE).

---

## 中文

本机 coding agent 的 token 用量。一行安装：

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

然后输入 `wheretoken`：有账本以来的总用量（M）、命中率、最长/当前连烧、请求、用户回合，再按工具和厂家排。`wheretoken serve` 仍是窑墙观察台（`127.0.0.1`）。不写美元价，不做遥测，不打印 JWT。工具 ≠ 厂家。已有 Go 时也可以 `go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest`。

尚未：GitHub 二进制未签名、没有 npm；Trae / Cursor 的 token 列需要在那些应用里已登录。
