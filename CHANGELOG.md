# Changelog

## Unreleased

CLI: `wheretoken` prints a character table (total M, cache hit rate, streaks, requests, turns), then tool/vendor rankings with share and a 7-day spark.

- Install: `go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest`, `scripts/install.sh`, npm wrapper, `brew install --HEAD Formula/wheretoken.rb`
- `wheretoken serve` still serves the kiln dashboard on `127.0.0.1`
- `--today`, `--tool`/`--vendor`/`--model`, brand flags, `--json`, `--ascii`, `--no-color`, `--quiet`, `--offline`, `--width`
- Exit `0` ok (including zero data / degraded login), `1` runtime, `2` usage
- Never prints JWTs or access tokens
- `wheretoken serve` sets `ReadHeaderTimeout`; go.mod pins `toolchain go1.25.13` for stdlib TLS/HTTP fixes
- `--offline` prints a banner under the title so a long ranking does not hide that the scan skipped Cursor/Trae APIs
