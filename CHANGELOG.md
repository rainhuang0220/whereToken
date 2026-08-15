# Changelog

## Unreleased

CLI: `wheretoken` prints a character table (total M, cache hit rate, streaks, requests, turns), then tool/vendor rankings with share and a 7-day spark.

- Install: `go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest`, `scripts/install.sh`, `scripts/install.ps1`, npm wrapper, `brew install --HEAD Formula/wheretoken.rb`
- `wheretoken serve` still serves the kiln dashboard on `127.0.0.1`
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
- `--help` examples include `--model=k3`
- Man page EXAMPLES; fixture script asserts COLUMNS=40 still spells Kimi
- Cursor and Trae HTTP clients have a 20s timeout contract test
- Redirects off Cursor/Trae API hosts are rejected (loopback too, unless it is APIBase)
- `wheretoken sources` with no ledgers keeps stdout empty and hints on stderr
- Flags may precede a subcommand (`wheretoken --home DIR sources`)
- Redact `x-goog-api-key` the same way as other vendor API-key headers
- Ranking tables treat rocket/extended emoji and Hangul as double-width
- `make ci` runs fmt-check, vet, tests, race, and the fixture CLI
- `--json` publishes `docs/cli-json.schema.json`; goreleaser builds .deb/.rpm with man + completions
