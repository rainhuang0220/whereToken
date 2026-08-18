<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  <b>Local-first token usage analytics for coding agents.</b>
</p>

<p align="center">
  Track token usage across Claude Code, Codex, Kimi Code,<br>
  Cursor, OpenCode, Grok CLI, and Trae.
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
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken dashboard home" width="900">
</p>

<p align="center">
  <sub>Dashboard home — <b>墨</b>, a monochrome newspaper-inspired theme.</sub>
</p>

Your coding-agent usage, aggregated locally from data already stored on your machine. No cloud sync. No telemetry.

## Features

### Unified usage overview

See token usage from every supported agent that already has a ledger on this computer.

### Agent and provider breakdown

Separate usage by coding agent, model provider, and model.

### Historical usage

Daily totals, streaks, and cache hit rate.

### Local dashboard

A browser UI that runs entirely on your machine.

### CLI

Query usage from the terminal, or export it as JSON.

whereToken reports **token counts**, not dollar estimates. Subscription plans and provider prices do not map cleanly onto a local ledger.

## Installation

### Homebrew

```bash
brew tap rainhuang0220/wheretoken
brew install wheretoken
```

### macOS / Linux

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

### Windows

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

The script prints the installed path (usually `~/.local/bin/wheretoken`). Run that line. Open a new terminal if the command is not on `PATH` yet.

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

The dashboard is included in GitHub Release binaries and `brew tap`. `go install` and `brew --HEAD` embed a stub. From a clone: `cd web && npm run build`, then `WHERETOKEN_WEB=web/dist wheretoken serve`.

The `npm/` wrapper is **not on the npm registry** yet.

## Quick start

```bash
wheretoken
```

<p align="center">
  <img src="docs/media/cli-kpi.png" alt="whereToken CLI summary" width="720">
</p>

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="Usage by agent"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="Usage by provider"></td>
  </tr>
  <tr>
    <td align="center"><sub>By agent</sub></td>
    <td align="center"><sub>By provider</sub></td>
  </tr>
</table>

```bash
wheretoken --today
wheretoken --cursor
wheretoken --vendor=anthropic
wheretoken serve
```

The dashboard is at [http://127.0.0.1:8787](http://127.0.0.1:8787) on this computer. **刷新** rescans. Reloading the tab does not.

See [`docs/wheretoken.1`](docs/wheretoken.1) for the full CLI reference.

## Dashboard

whereToken includes a local web dashboard for exploring usage across supported agents. The UI is currently Chinese-first.

**窑** is the project mascot: a small furnace, for tokens consumed over time.

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑, the whereToken mascot" width="900">
</p>

## Supported coding agents

whereToken reads usage information already stored by your coding agents. It does not invent numbers when the source data is missing.

| Agent | Usage data | Sign-in |
|-------|------------|---------|
| Claude Code | Full | Not required |
| Kimi Code | Full | Not required |
| Codex | Full | Not required |
| OpenCode | Full | Not required |
| Grok CLI | Full | Not required |
| Cursor | Partial | Required for token columns. The app must be **signed in** |
| Trae / Trae CN / TRAE SOLO | Partial | Required. Encrypted `storage.json` is reported, not decrypted |

See [`docs/data-sources.md`](docs/data-sources.md) for how each agent is read.

Not currently supported: Windsurf, GitHub Copilot, Cline, and Lingma. These tools do not currently expose token usage through a local data source that whereToken can reliably read.

## Agent and provider

whereToken distinguishes the **coding agent** you use from the **provider** that served the model.

A request made through Claude Code using a MiniMax model is attributed as:

- **Agent:** Claude Code
- **Provider:** MiniMax

Some agents expose token data only when the application is signed in. whereToken does not treat unavailable data as zero.

## Privacy & Security

whereToken is designed to run locally.

### Data collection

whereToken does not collect or upload your usage data, session history, or credentials to a whereToken server. There is no telemetry and no cloud sync.

### Local data

It reads usage information from files already stored on this computer by supported coding agents.

### Network access

The dashboard is served locally and does not require a remote whereToken service. It is not exposed to other devices on your network. The address is [http://127.0.0.1:8787](http://127.0.0.1:8787).

Most agents are files only. Cursor and Trae may contact **their** hosts using login state those apps already stored locally, to fill token columns the local ledger does not have.

### Credentials

whereToken does not ask you for API keys. When an integration needs authentication, it uses data or credentials already managed by that application.

### Reporting issues

Do not include API keys, session tokens, JWTs, cookies, or other secrets in bug reports. The CLI never prints those values to stdout. See [`SECURITY.md`](SECURITY.md).

## Limitations

whereToken is currently in **alpha** (v0.3.0). It is usable; some integrations are still evolving.

- Cursor and Trae token support is currently limited
- macOS GitHub binaries are currently **unsigned** ([`docs/macos-signing.md`](docs/macos-signing.md))
- npm distribution is **not on the npm registry** yet
- The dashboard UI is currently Chinese-first

## Documentation

- CLI reference: [`docs/wheretoken.1`](docs/wheretoken.1)
- Data sources: [`docs/data-sources.md`](docs/data-sources.md)
- JSON schema: [`docs/cli-json.schema.json`](docs/cli-json.schema.json)
- Completions: [`completions/`](completions/)

## Development

```bash
go test ./...
make test
make ci
bash scripts/verify-cli.sh
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

Toolchain is **Go 1.25.13** from `go.mod`. CI runs on Ubuntu, macOS, and Windows ([`ci/github-workflows/`](ci/github-workflows/)).

## License

[MIT](LICENSE).
