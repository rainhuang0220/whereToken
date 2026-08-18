<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  See where your coding-agent tokens went.
</p>

<p align="center">
  <a href="./README.md"><b>English</b></a> ·
  <a href="./README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <a href="https://github.com/rainhuang0220/whereToken/releases"><img src="https://img.shields.io/github/v/release/rainhuang0220/whereToken?include_prereleases&style=flat-square&color=FFD700&label=alpha" alt="alpha"></a>
  <a href="https://github.com/rainhuang0220/whereToken/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rainhuang0220/whereToken/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/rainhuang0220/whereToken?style=flat-square" alt="MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/rainhuang0220/whereToken?style=flat-square" alt="Go"></a>
  <img src="https://img.shields.io/badge/platform-macOS%20%7C%20Linux%20%7C%20Windows-informational?style=flat-square" alt="macOS Linux Windows">
</p>

<p align="center">
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken dashboard" width="900">
</p>

whereToken is a **local-first** CLI (and optional dashboard) for token usage across coding agents. It reads data already on your machine.

**Claude Code · Kimi Code · Codex · Cursor · OpenCode · Grok CLI · Trae**

Your usage data stays on your machine. No cloud sync. No telemetry.

<p align="center">
  <img src="docs/media/cli-kpi.png" alt="whereToken CLI" width="720">
</p>

## Features

- Total usage across every agent that already has a ledger on disk
- Breakdown by **tool**, **vendor**, and **model**
- Today, streaks, and cache hit rate
- JSON for scripts
- Optional local dashboard
- Token counts only — it does not estimate USD, because subscriptions and provider prices do not map cleanly onto a local ledger

## Install

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

curl / irm prints the path it installed (usually `~/.local/bin/wheretoken`). Run that line. Open a new terminal if the command is not on `PATH` yet.

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

The dashboard is in GitHub Release binaries and `brew tap`. `go install` / `brew --HEAD` embed a stub — from a clone: `cd web && npm run build`, then `WHERETOKEN_WEB=web/dist wheretoken serve`.

The `npm/` wrapper is **not on the npm registry** yet.

## Quick start

```bash
wheretoken
```

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="Usage by tool"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="Usage by vendor"></td>
  </tr>
  <tr>
    <td align="center"><sub>By tool</sub></td>
    <td align="center"><sub>By vendor</sub></td>
  </tr>
</table>

```bash
wheretoken --today
wheretoken --cursor          # also --claude --kimi --grok --codex --opencode --trae
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken --json
wheretoken --offline         # skip Cursor / Trae APIs
wheretoken serve
```

Then open [http://127.0.0.1:8787](http://127.0.0.1:8787) on this computer. **刷新** rescans. Reloading the tab does not.

`wheretoken --help` has the rest. Units are **M** (million tokens).

## Dashboard

The page above is **墨** — black and white, like a newspaper. It is the author's favorite.

**窑** is where the name comes from. Tokens burn fast, like a kiln. The mascot is that little gold furnace.

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑 theme — the little kiln the mascot comes from" width="900">
</p>

## Supported tools

Only tools that already have a ledger on disk. Field mapping: [`docs/data-sources.md`](docs/data-sources.md).

| Tool | Token data | Login required? |
|------|------------|-----------------|
| Claude Code | Yes | No |
| Kimi Code | Yes | No |
| Grok CLI | Yes | No |
| Codex | Yes | No |
| OpenCode | Yes | No |
| Cursor | Partial — local bubbles still count requests / turns | **Yes**, for token columns. App must be **signed in** |
| Trae / Trae CN / TRAE SOLO | Partial | **Yes**. Encrypted `storage.json` is reported, not decrypted |

Not in this release: Windsurf, Copilot, Cline, Lingma, and similar.

## Tool vs. vendor

whereToken separates the **app you typed in** from the **model provider** behind it.

A request through Claude Code that used a MiniMax model is:

- **Tool:** Claude Code
- **Vendor:** MiniMax

So you can see both where you asked and who actually served it.

## Privacy

whereToken is local-first.

It reads usage data from files already on this computer. It does not upload those files, does not sync them to a whereToken server, and does not phone home.

It never asks you for an API key. It does not send your credentials to whereToken.

Most sources are files only. Cursor and Trae may use login state those apps already stored locally, to fill token columns the ledger does not have. That traffic goes to **their** hosts, not to whereToken.

The optional dashboard also runs on this computer. It is not exposed to other devices on your network.

## Security

The CLI never prints JWTs, access tokens, API keys, or cookies. Do not paste secrets into issues. [`SECURITY.md`](SECURITY.md) has the rest.

## Limitations

**v0.3.0 is an Alpha.** It works on a real machine. Some integrations are still evolving. The UI is Chinese for now.

- macOS GitHub binaries are currently **unsigned** ([`docs/macos-signing.md`](docs/macos-signing.md))
- There is **no npm package**
- Cursor / Trae token columns need those apps **signed in** on this machine. Claude / Kimi / Grok / Codex / OpenCode do not

<details>
<summary>Flags, env, exit codes</summary>

```bash
wheretoken --ascii
wheretoken --width 40
wheretoken --quiet
wheretoken sources
wheretoken scan --json       # dashboard 刷新 dump; not schema 1; no --today / --tool
wheretoken completion zsh    # also bash, fish, powershell
```

Completions: [`completions/`](completions/). Man page: [`docs/wheretoken.1`](docs/wheretoken.1). JSON: [`docs/cli-json.schema.json`](docs/cli-json.schema.json).

| Env | |
|-----|--|
| `NO_COLOR` / `FORCE_COLOR` | color (`NO_COLOR` wins) |
| `WHERETOKEN_ASCII=1` / `NO_UTF8` | ASCII boxes |
| `WHERETOKEN_OFFLINE=1` | same as `--offline` |
| `WHERETOKEN_HOME` / `--home` | fake home (tests) |
| `WHERETOKEN_EXTRA_ROOTS` | extra homes (`:` / `;` / commas) |
| `WHERETOKEN_WEB` | `web/dist` for `serve` |
| `CODEX_HOME` | Codex also reads this |
| `COLUMNS` | same as `--width` |

Exit: `0` ok (including empty or a degraded login), `1` fail, `2` usage.

</details>

## Development

```bash
go test ./...
make test
make ci
bash scripts/verify-cli.sh
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

Toolchain is **Go 1.25.13** from `go.mod`. CI: Ubuntu / macOS / Windows ([`ci/github-workflows/`](ci/github-workflows/)).

## License

[MIT](LICENSE).
