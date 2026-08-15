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
