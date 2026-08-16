# Changelog

## 0.1.1 — 2026-08-17

Query and install polish for a stranger's first run.

- Empty home prints a Chinese footnote instead of a silent zero table; `--today` with no rows says so, and `--today --kimi` (when all-time Kimi exists) hints to drop `--today` instead of mixing all-time rows
- Unknown vendor footnote is `未知厂家`, not mixed `Unknown 厂家`
- Unknown flags exit 2 as `unknown flag "--name"` (not Go's `flag provided but not defined`)
- `--help` examples include `--today --kimi`; INSTALL says no tap, no npm, unsigned binaries
- README has a short **Not yet** list (unsigned, no tap, no npm, Trae/Cursor need those apps signed in)
- `Formula/wheretoken.rb` pins the `v0.1.1` source tarball SHA256 and still offers `--HEAD`

## 0.1.0 — 2026-08-16

CLI: `wheretoken` prints a character table (total M, cache hit rate, streaks, requests, turns), then tool/vendor rankings with share and a 7-day spark.

- Install: `curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash` (Windows: `irm …/install.ps1 | iex`). `go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest` if you already have Go. Homebrew: `brew install --HEAD ./Formula/wheretoken.rb`. The npm package is not on the registry.
- GitHub Release archives (darwin/linux/windows, amd64/arm64) plus `checksums.txt` (SHA-256)
- `wheretoken serve` still serves the kiln dashboard on `127.0.0.1`; `--offline` / `WHERETOKEN_OFFLINE` skip Cursor/Trae APIs on dashboard 刷新 too
- `--today`, `--tool`/`--vendor`/`--model`, brand flags, `--json`, `--ascii`, `--no-color`, `--quiet`, `--offline`, `--width`
- Exit `0` ok (including zero data / degraded login), `1` runtime, `2` usage
- Never prints JWTs or access tokens
- `wheretoken serve` sets `ReadHeaderTimeout`; go.mod pins `toolchain go1.25.13` for stdlib TLS/HTTP fixes
- `--json` includes raw `total` on each tool/vendor row (not only `total_m`)
- ASCII rankings use `...`, not a Unicode ellipsis
- `--today` keeps the offline note; a slice with requests but 0 tokens says so in plain language
- Windows: `scripts/install.ps1`; Homebrew formula installs bash/zsh/fish completions
- Narrow terminals drop ranking columns (回合, then 请求) instead of turning every name into `C...`
- `--cursor` / `--tool` slices no longer foot-note an unused Trae login
- Hide the model ranking when every model is 0.00 M (offline Cursor was a 40-row zero dump)
- Redact `openai-api-key` / `anthropic-api-key` headers the same way as JWTs
- CI copies pin GitHub Actions to commit SHAs (checkout v4.2.2, setup-go v5.5.0, setup-node v4.4.0)
- `install.sh` / `install.ps1` / npm postinstall verify `checksums.txt` (SHA-256) before installing a GitHub Release binary
- `wheretoken scan --json` and `/api/scan` redact JWTs in error strings; serve sends `X-Content-Type-Options: nosniff` and `X-Frame-Options: DENY`
- Reject `--width < 0` and out-of-range `--port` as usage (exit 2)
- `--model` slices do not inherit the tool's user-turn count (turns have no model id); the table shows — not 0
- `--model=k3` matches `kimi-code/k3`; `--json` sets `hide_turns` when that KPI is not meaningful
- Fish completion lists vendor ids the same way zsh already did
- Honor `WHERETOKEN_HOME` through the CLI's env lookup (tests and `--home` stay fake-home only)
- CI runs `gofmt -l`; `make fmt-check` does the same locally
- `--help` examples include `--model=k3`; INSTALL leads with curl / irm, then `go install`
- `install.sh` / `install.ps1` fall back to `go install` into PREFIX when no GitHub Release exists (and do not print the GitHub 404)
- Man page EXAMPLES; fixture script asserts COLUMNS=40 still spells Kimi
- Cursor and Trae HTTP clients have a 20s timeout contract test
- Redirects off Cursor/Trae API hosts are rejected (loopback too, unless it is APIBase)
- `wheretoken sources` with no ledgers keeps stdout empty and hints on stderr
- Flags may precede a subcommand (`wheretoken --home DIR sources`) and also follow it (`wheretoken --offline serve --port 8799`)
- Redact `x-goog-api-key` the same way as other vendor API-key headers
- Ranking tables treat rocket/extended emoji and Hangul as double-width
- Narrow terminals wrap the legend, offline banner, and footnotes instead of letting them spill
- 40-column wrap breaks after ideographic commas, keeps `Cursor`/`token` whole, and hanging-indents so a middle-dot does not look like a second bullet
- `make ci` runs fmt-check, vet, tests, race, and the fixture CLI
- `--json` publishes `docs/cli-json.schema.json`; goreleaser builds .deb/.rpm with man + completions
