<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  <b>Local-first token usage analytics for coding agents.</b>
</p>

<p align="center">
  Track token usage across Claude Code, Kimi Code, Codex, Cursor,<br>
  OpenCode, Grok CLI, Trae, and other supported tools.
</p>

<p align="center">
  <a href="./README.md"><b>English</b></a> ·
  <a href="./README.zh-CN.md">简体中文</a>
</p>

<p align="center">
  <img src="https://img.shields.io/badge/status-alpha-FFD700?style=flat-square" alt="Status: Alpha">
  <a href="https://github.com/rainhuang0220/whereToken/releases"><img src="https://img.shields.io/github/v/release/rainhuang0220/whereToken?include_prereleases&style=flat-square" alt="release"></a>
  <a href="https://github.com/rainhuang0220/whereToken/actions/workflows/ci.yml"><img src="https://img.shields.io/github/actions/workflow/status/rainhuang0220/whereToken/ci.yml?branch=main&style=flat-square&label=CI" alt="CI"></a>
  <a href="./LICENSE"><img src="https://img.shields.io/github/license/rainhuang0220/whereToken?style=flat-square" alt="MIT"></a>
  <a href="https://go.dev/"><img src="https://img.shields.io/github/go-mod/go-version/rainhuang0220/whereToken?style=flat-square" alt="Go"></a>
</p>

<p align="center">
  <img src="docs/media/dash-newspaper.jpg" alt="whereToken dashboard" width="900">
</p>

<p align="center">
  <sub><b>墨</b> is the monochrome, newspaper-style theme.</sub>
</p>

whereToken reads usage data from sources already stored on your machine and is designed to operate locally. Feedback and bug reports are welcome.

## Features

- Aggregate token usage across supported coding agents
- Break usage down by agent, provider, and model
- View daily totals, streaks, and cache hit rate
- Export usage as JSON
- Explore the same data in a local dashboard

whereToken reports token counts. It does not estimate monetary cost.

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

### From source

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
```

Release binaries and `brew tap` include the dashboard. `go install` and `brew --HEAD` build the CLI only. To serve the dashboard from a clone, build the web UI first (`cd web && npm run build`) and set `WHERETOKEN_WEB` to `web/dist`.

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
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken --json
wheretoken --offline
```

Run `wheretoken --help` for the complete command reference.

## Dashboard

Start the local dashboard with:

```bash
wheretoken serve
```

The dashboard runs locally on your machine. It provides a visual overview of token usage across supported coding agents, providers, and models. Use the refresh control in the page to rescan; reloading the browser tab does not.

**窑** is whereToken's furnace mascot, representing the idea of tokens being consumed over time.

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑, the whereToken mascot" width="900">
</p>

## Supported coding agents

whereToken reads usage information from data made available by supported coding agents. Availability varies by tool and may depend on whether the application is signed in.

| Coding agent | Token data | Authentication |
|--------------|------------|----------------|
| Claude Code | Full | Not required |
| Kimi Code | Full | Not required |
| Grok CLI | Full | Not required |
| Codex | Full | Not required |
| OpenCode | Full | Not required |
| Cursor | Partial | Required for token columns |
| Trae / Trae CN / TRAE SOLO | Partial | Required |

Cursor and Trae need those applications **signed in** on this machine for token columns. Encrypted Trae storage is reported, not decrypted. Unavailable data is not treated as zero.

See [`docs/data-sources.md`](docs/data-sources.md) for how each agent is read.

### Not currently supported

Windsurf, GitHub Copilot, Cline, and Lingma are not currently supported because whereToken does not yet have a reliable local usage source for these tools.

## How it works

whereToken distinguishes between the coding agent you use and the provider serving the model.

For example, a request made through Claude Code using a MiniMax model is attributed to:

- **Agent:** Claude Code
- **Provider:** MiniMax

## Privacy & Security

### Data collection

whereToken does not collect or upload usage data, session history, or credentials to a whereToken service. There is no telemetry.

### Local data

whereToken reads usage information from data already stored on your computer by supported coding agents.

### Network access

The optional dashboard runs locally on your machine and does not require a remote whereToken service. It is not exposed to other devices on your network.

Most agents are read from local files only. Cursor and Trae may contact those applications' own hosts using login state they already stored locally, to obtain token columns that are not available from the local files alone.

### Credentials

whereToken does not ask users to provide API keys directly. When an integration requires authentication, it uses local data or credentials already managed by the corresponding application.

### Security

For security issues and the project's security policy, see [`SECURITY.md`](SECURITY.md). Do not include API keys, session tokens, or other secrets in bug reports.

## Limitations

whereToken is currently in **alpha**.

- GitHub release binaries are currently **unsigned** ([`docs/macos-signing.md`](docs/macos-signing.md))
- An npm package is not currently published
- Cursor and Trae token data requires those applications to be signed in
- The dashboard UI is currently Chinese-first

## Documentation

- CLI reference: [`docs/wheretoken.1`](docs/wheretoken.1)
- Data sources: [`docs/data-sources.md`](docs/data-sources.md)
- JSON output format: [`docs/cli-json.schema.json`](docs/cli-json.schema.json)
- Completions: [`completions/`](completions/)

For the complete CLI reference, environment variables, exit codes, and JSON output format, see the project documentation.

## Development

```bash
go test ./...
make test
make ci
```

```bash
cd web && npm install && npm test && npm run build
go run ./cmd/wheretoken serve
```

## License

[MIT](LICENSE).
