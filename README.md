<p align="center">
  <img src="docs/media/logo.png" width="96" alt="whereToken kiln kid">
</p>

<h1 align="center">whereToken</h1>

<p align="center">
  See where your <b>local</b> coding-agent tokens went.<br>
  One command. A character table. A kiln if you want the wall.
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
  <img src="docs/media/cli-kpi.png" alt="wheretoken — eight figures since records began" width="720">
</p>

> **v0.3.0 Alpha.** Try it, [file an issue](https://github.com/rainhuang0220/whereToken/issues), send a PR. UI is Chinese for now.

Reads ledgers already on this machine. No cloud, no USD, no telemetry.

**Tool ≠ vendor** — Claude Code running MiniMax stays Claude Code / MiniMax. A signed-out Cursor is a footnote, not a fake zero.

<table>
  <tr>
    <td width="50%" valign="top"><img src="docs/media/cli-tools.png" alt="Ranking by tool"></td>
    <td width="50%" valign="top"><img src="docs/media/cli-vendors.png" alt="Ranking by vendor"></td>
  </tr>
  <tr>
    <td align="center"><sub>By tool</sub></td>
    <td align="center"><sub>By vendor</sub></td>
  </tr>
</table>

## Install

```bash
brew tap rainhuang0220/wheretoken && brew install wheretoken
```

```bash
curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
```

```powershell
irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
```

curl / irm prints the binary path (usually `~/.local/bin/wheretoken`). Run that. Open a new terminal if `wheretoken` is not on `PATH` yet.

```bash
go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest   # Go 1.25+
```

The dashboard ships in GitHub Release and `brew tap`. `go install` / `brew --HEAD` get a stub — from a clone: `cd web && npm run build`, then `WHERETOKEN_WEB=web/dist wheretoken serve`.

The `npm/` wrapper is **not on the npm registry** yet.

## Usage

```bash
wheretoken                 # since records began
wheretoken --today         # local timezone
wheretoken --cursor        # also --claude --kimi --grok --codex --opencode --trae
wheretoken --vendor=xai
wheretoken --model=k3
wheretoken --json          # schema 1
wheretoken --offline       # skip Cursor / Trae APIs
wheretoken serve           # http://127.0.0.1:8787
```

`wheretoken --help` has the rest. Units are **M** (million tokens).

## Dashboard

```bash
wheretoken serve           # http://127.0.0.1:8787
```

**刷新** rescans. A tab reload does not.

**墨** is the newspaper look — the author's favorite.

<p align="center">
  <img src="docs/media/dash-newspaper.jpg" alt="Home in 墨, the newspaper theme" width="900">
</p>

**窑** is the name. Tokens burn fast, like a kiln. The mascot is that little gold furnace.

<p align="center">
  <img src="docs/media/dash-kiln.png" alt="窑 — the little kiln the mascot comes from" width="900">
</p>

## Sources

Only tools that already have a ledger on disk. Paths: [`docs/data-sources.md`](docs/data-sources.md).

| Source | Tokens without signing in? |
|--------|----------------------------|
| Claude Code | Yes |
| Kimi Code | Yes |
| Grok CLI | Yes |
| Codex | Yes |
| OpenCode | Yes |
| Cursor | Requests / turns locally. **Token columns need the app signed in.** |
| Trae / Trae CN / TRAE SOLO | **Sign-in.** Encrypted `storage.json` is reported, not decrypted. |

No Windsurf / Copilot / Cline / Lingma until they leave a clear local token ledger.

`wheretoken scan --json` is the dashboard **刷新** dump — not schema 1, no `--today` / `--tool`.

<details>
<summary>Flags, env, exit codes</summary>

```bash
wheretoken --ascii
wheretoken --width 40
wheretoken --quiet
wheretoken sources
wheretoken completion zsh   # also bash, fish, powershell
```

Completions: [`completions/`](completions/). Man page: [`docs/wheretoken.1`](docs/wheretoken.1). JSON schema: [`docs/cli-json.schema.json`](docs/cli-json.schema.json).

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

## Privacy

Local files only. `serve` binds `127.0.0.1`. No telemetry, no third-party fonts, no JWTs on stdout. Don't paste secrets into issues. [`SECURITY.md`](SECURITY.md).

## Not yet

- GitHub binaries are **unsigned** ([`docs/macos-signing.md`](docs/macos-signing.md)).
- **No npm package.**
- Trae / Cursor **token columns** need those apps **signed in**. Claude / Kimi / Grok / Codex / OpenCode do not.

## Develop

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
