# Changelog

## Unreleased

- Scan HUD is a 4-line kiln kid (tuft + face + pot) that fidgets on its own clock: scratch / abacus / toss / fire / blink / grin
- Caption stays lemon, the mood (`挠头` / `拨算盘` / `投煤` / `煅烧`) is dim — not one flat wash
- `--ascii` keeps `(o_o)` / `(^_^)` and English moods
- Title color is lemon `228`, not Claude-orange `208`
- An empty home puts the blinking kid above the zero table: 窑里还是冷的
- Dashboard firing veil draws the same kid as an SVG; the title rail keeps a small idle one

## 0.2.0 — 2026-08-18

The first-run character table. Same code as 0.1.2 plus a one-line install next-command.

- After `curl | bash`, the only next command is the installed path (`~/.local/bin/wheretoken`)
- KPI total is bold, hit rate is green/yellow/red, and the title uses kiln orange (TTY only; `NO_COLOR` still wins)
- `install.sh` writes `~/.local/bin` into `~/.zshrc` so the next terminal finds `wheretoken`

## 0.1.2 — 2026-08-18

Colored CLI table, first-run copy, and install PATH. Superseded the same day by 0.2.0.

- KPI total is bold, hit rate is green/yellow/red, and the title uses kiln orange (TTY only; `NO_COLOR` still wins)
- `install.sh` writes `~/.local/bin` into `~/.zshrc` so the next terminal finds `wheretoken`
- Trae account fetch overlaps session requests (8 at a time) and gives up after 30s instead of walking 500 × 20s
- Hitting Trae’s 500-session cap still keeps those 500 rows; the error is a footnote, not a zeroed Trae
- GET `/api/summary` marks `"scanning": true` while a 刷新 is running; a 409 刷新 waits for that scan instead of leaving a dead kiln
- A 409 still explains itself when the previous kiln is on screen
- `--help` says `scan` is observatory JSON, not schema 1, and that `go install` / `brew --HEAD` are the CLI table
- `wheretoken completion zsh --quiet` is valid (flags after the shell name)
- Empty-home `--today` uses the same “本机没有找到账本” line as the all-time table
- `POST /api/scan` / `GET /api/summary` refuse a foreign `Origin` / `Referer`
- From a clone, `go run` uses `./web/dist` only when `go.mod` is this module; README says `WHERETOKEN_WEB`
- OpenCode rows without `time.created` stay undated (not 1970)
- Same-millisecond Kimi `usage.record` rows stay two requests
- Redact `X-Cloudide-Token` and JSON `access_token` / `refresh_token` fields
- The KPI box shrinks with `--width` instead of overflowing a 40-column terminal
- All-time `--json` keeps `max_streak_days` / `current_streak_days` when they are 0 (`--today` still omits them)
- Missing `--tool` / invalid `--port` say so without Go's `flag needs an argument` / `parse error`
- 窑墙轴 skips absent tools, is one tab stop, and moves with arrow keys
- Completions after `scan` / `serve` / `completion` no longer offer table filters those commands reject
- Kiln bricks are one tab stop; arrow keys move on the 7-row week grid
- Cursor account pagination continues when a page’s parsed rows are fewer than the raw list (and fewer than 1000)
- Trae keeps the current session, then memento list order, so the 500-session cap is stable
- `--help` says `--json` is schema 1 and `scan` is a different dump
- A plaintext JWT in Trae `storage.json` is used; only non-JSON blobs stay “encrypted storage”
- Empty first 刷新 hides hollow ranking / drill tables; KPI readout is 2×3 below 1100px
- Homebrew tap [`rainhuang0220/wheretoken`](https://github.com/rainhuang0220/homebrew-wheretoken) installs the GitHub Release binary (`brew tap rainhuang0220/wheretoken` then `brew install wheretoken`); no Go required
- Release workflow signs and notarizes macOS binaries when `MACOS_SIGN_P12` and the other Apple secrets are set; otherwise it still publishes unsigned archives ([`docs/macos-signing.md`](docs/macos-signing.md))
- `wheretoken serve` prints that 刷新 rescans and that reloading the tab does not
- An empty kiln visit says the same “本机没有找到账本” line as the CLI table
- `scan --json` / `/api/scan` mark `"offline": true` when Cursor/Trae APIs were skipped; the dashboard shows the same offline banner as the table
- Unknown vendor rows are labeled `未知厂家`, not `Unknown`
- Dashboard request and turn counts use the same thousands grouping as the CLI table
- Kiln bricks are keyboard-focusable and announce the same two-line caption as pointer hover
- `GET /api/summary` before the first 刷新 still includes a Monday-based kiln window
- `/api/scan` and `/api/summary` refuse a non-localhost `Host`
- `wheretoken --help` and the man page say 刷新 rescans; man page documents `COLUMNS` / `FORCE_COLOR` / `NO_UTF8`
- Empty `Host` on a localhost bind is still allowed (old HTTP clients)
- Error strings redact `Cookie` / `Set-Cookie` the same way as JWTs
- Cursor / Trae / OpenCode open SQLite through one escaped `file:` URI so `?` and `#` in the path cannot steal the query
- Axis damper aria-label is `窑墙轴`, not `消耗轴`
- GitHub Release archives include `docs/wheretoken.1` (not only the .deb/.rpm)
- The kiln dashboard no longer loads Google Fonts (local-first; system fallbacks stay in the glaze stacks)
- `grok-*` / xAI models map to vendor `xai` (label `xAI`) instead of 未知厂家
- Linux CI runs `scripts/govulncheck.sh`
- Dashboard 刷新 treats an SSE `error` event as a failure instead of hanging as “scan incomplete”
- `--help` vendor list includes `xai`
- Chinese README says 刷新 rescans and that the dashboard does not load third-party fonts
- Tool and vendor table rows are keyboard-activatable (Enter/Space); absent rows stay inert
- Trae billing JSON no longer double-counts a parent total and its session leaves, and two models in one session sum instead of taking max
- Codex `last_token_usage` followed by a matching `total_token_usage` is one request, not two
- Trae rows without a timestamp stay undated so the kiln does not dump them on today
- 工具 × 厂家 includes 缓存写; session drill request counts use thousands grouping
- Authoritative Cursor tokens footnote the 53-week account window
- Cursor account events with the same millisecond and model keep both conversations
- Linux CI runs `go vet` the same way `make ci` does
- Trae CN JWT files pick the CN API host even when the discovered product folder is named `Trae`
- A failed first 刷新 (including 409 煅烧进行中) explains what to do instead of leaving a silent cold kiln
- `~/.kimi` symlinked to `~/.kimi-code` is one Kimi root, not two
- `WHERETOKEN_EXTRA_ROOTS` that only aliases the same home (symlink) is not scanned twice
- Empty-home `--claude` / `--model=k3` explain themselves instead of a silent zero table or exit 2
- `--today` keeps Trae/Cursor login notes
- Claude stream rows without `requestId` are skipped so they cannot inflate cache_read
- `wheretoken serve` no longer prefers `./web/dist` from the current working directory
- Theme boot does not write `localStorage` until 应用
- `verify-cli.sh` clears `WHERETOKEN_EXTRA_ROOTS`
- `--help` says `go install` is the CLI table; kiln UI is in Release / brew
- Redact `Cloud-IDE-JWT` headers
- Default table width follows the TTY when `COLUMNS` is unset (bash does not export it)
- `wheretoken scan --today` is usage: observatory JSON does not take table filters
- Trae says so when it only fetched the first 500 sessions

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
