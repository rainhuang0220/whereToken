# Changelog

## Unreleased

- `verify-cli.sh` requires 估价/排名, rejects `#0` / `rank: 0`, and checks `--no-community`
- `--help` / man: `--rank all` is uploaded days, not kiln 全部
- 用量说明 drops a Community Rank podium when N is below 20 (`#1 / 3` cannot appear)
- README says Rank 累计 is uploaded days, not kiln 全部; `DO_NOT_TRACK=1` is listed next to community off
- Man page documents `doctor` Community Rank (no upload) and `DO_NOT_TRACK`
- Claude Code skips `feedback-bundles/` (full transcripts) and `stats-cache.json`
- All-time 用量说明 uses 累计 rank (uploaded days), not today's podium. The 累计 control says it is not kiln 全部
- `DO_NOT_TRACK=1` (also true/on/yes) turns Community Rank off the same way as `WHERETOKEN_COMMUNITY=off`; empty and `0` do not
- OpenClaw skips the per-agent runtime SQLite dir (`agent/`); JSONL archives still count
- SanitizeStanding and dashboard rankCaption hide `#n / m` when m < 20 so a remote payload cannot show a three-person podium
- xAI `grok-4` / `grok-4-fast` / `grok-4-latest` stay unpriced; prefix match treats `-`, `.`, or a following digit as a different model id
- Community Rank leave drops the participant's rate-limit hits and does not write an `opted_out` tombstone. Local leave invalidates the rank cache
- Community Rank CLI/dashboard copy: not a global / worldwide / all-AI-users rank; estimated cost is not a subscription bill
- `GET /api/summary` no longer uploads Community Rank. `serve --no-community` forbids dashboard join/leave and does not mint `community.json`
- `--json` and `/api/community` omit `$0.0000` and sanitize `#0`. Rank upload rejects an `estimated_cost_usd` that formats as `$0.0000`. Non-ok standing never keeps a podium. Dashboard `?since=today` is the local calendar day
- Community Rank `community.json` tests use testhome XDGConfig vs AppData (Unix and Windows); the man page names both defaults
- Community Rank leave of an unknown UUID matches never-seen (no `opted_out` oracle). Empty `ipHit` keys are evicted
- GET `/rank` after leave matches a never-seen UUID. Unconfigured `/api/community` stays disabled
- `community off` and dashboard 退出社区 stay enabled if remote leave fails. `o4-mini` uses the standard list ($1.10 / $0.275 / $4.40), not Batch/Flex
- Dashboard fifth cell keeps 估价 over 排名 and always shows the Community Rank footnote
- Dashboard complete 估价 footnotes that it is not a bill. `FormatUSD` omits `$0.0000` when a price is unavailable
- Dashboard treats `$0.0000` as unknown. Cost rank scores round to µUSD. Rank GET is rate-limited
- Cost footnotes skip a blank/zero bill when the priced amount rounds under `$0.0001`
- 用量说明 uses the same cost formatter as the KPI (sub-cent rounding to `$0.0000` is omitted)
- CLI tool/vendor/model rankings print 估价 (`—` when unpriced). Narrow terminals drop that column first
- 工具 × 厂家 prints 估价; a partial slice with no priced dollars omits `cost_usd` instead of `$0.0000`. Rank HTTP sanitizes `#0` before encode
- Session and workspace drill print 估价 the same way as models (`—` when unpriced). 用量说明 names a real Community Rank place, never `#0`
- Community Rank client accepts only http(s) URLs, refuses redirects, and rate-limits the fake rank server by connection IP
- `wheretoken doctor` reports Community Rank configuration without uploading; `--help` says so
- Sub-cent estimates that round to `$0.0000` are omitted; ApplyWindow clones Community Rank
- `--width` drops 估价 before 回合/请求
- Dashboard copies the last scan under the mutex so 刷新 and 退出社区 cannot tear Community
- `serve --no-community` skips rank Sync even when `WHERETOKEN_COMMUNITY_URL` is set
- Windowed observatory insights no longer paste today's Community Rank onto a 7d/today total
- `--json` sanitizes Community Rank so `status=ok` never ships with `rank: 0`
- Drill sessions fold into `(其余)` after 40 rows; `--offline` also skips community upload
- Vendor-axis workspace drill keeps user turns; 用量说明 is Chinese and notes it is window-all, not the kiln axis
- Unconfigured Community Rank no longer looks opted-in on the dashboard
- Hyphenated Anthropic ids (`claude-opus-4-6`) use the current Opus card, not retired Opus 4
- `wheretoken serve --no-community` actually disables rank upload; rank HTTP rejects dates outside a ±2 day window
- Community status redacts the participant UUID; leave is rate-limited
- Dashboard fifth column keeps 估价 over 排名 in one cell on narrow layouts
- Drill JSON omits `cost_usd` when a slice has no list price; usage insights skip unlabeled model/session buckets
- Estimated API-equivalent cost stays in the CLI/dashboard fifth column (row 1); unknown prices stay `—`, never `$0`
- Community Rank (row 2): anonymous daily totals, Today/All, competition ranking, `#n / N`; not a global / worldwide / all-AI-users rank
- `wheretoken community status|on|off|serve`, `--rank`, `--no-community`, `WHERETOKEN_COMMUNITY_URL`
- Privacy docs: local-first remains the core; no public `WHERETOKEN_COMMUNITY_URL` (remote deploy blocker); rank uploads aggregates only; estimated cost is API-equivalent, missing ≠ zero (`docs/community.md`)

## 0.4.3 — 2026-08-19 (Alpha)

- Windows cmd one-line install: `curl.exe -fsSL -o %TEMP%\wt-install.cmd …/install.cmd && %TEMP%\wt-install.cmd`
- `wheretoken update` / `uninstall` replace or remove a running Windows `.exe`

## 0.4.2 — 2026-08-19 (Alpha)

- OpenClaw session JSONL is re-read: an earlier empty index cache no longer sticks at 0
- Trae CN login is read from the encrypted `storage.json` blob (same format Trae uses). Missing `trae-jwt-token` is no longer treated as logged out

## 0.4.1 — 2026-08-19 (Alpha)

- Estimated API-equivalent cost (`docs/cost.md`): unknown prices stay unavailable, never `$0`
- Price card matches public 2026-08-19 list pages (retired Opus 4 / 4.1, GPT-5.6 cache writes, xAI short-context only)
- MiniMax Agent local ledger (`~/.minimax/v2/sqlite/runtime-state.sqlite` `local_runtime_token_usage`)
- OpenClaw session JSONL (`~/.openclaw/agents/*/sessions/*.jsonl`; not trajectories)
- MiniMax M2.1 / M2.5 / M2.7 pay-as-you-go list prices; M3 stays unavailable (context-tiered)
- Vendor-axis session drill keeps user turns; model drill prints server cost_usd
- OpenClaw top-level numeric timestamps still count; MiniMax NULL model rows no longer abort the ledger
- MiniMax sqlite replay stays unchanged; OpenClaw JSONL appends incrementally and truncations rescan
- Deterministic usage insights on the observatory payload and dashboard
- Dashboard period clicks ignore stale `/api/summary` replies
- README no longer claims the product does not estimate cost

## 0.4.0 — 2026-08-19 (Alpha)

- `wheretoken update` replaces this binary with the latest GitHub Release (`brew upgrade` when Homebrew owns it)
- `wheretoken uninstall` removes this binary (`brew uninstall` when Homebrew owns it)
- Incremental JSONL stores the last consumed newline offset, so an unfinished last line is reread when the writer finishes it
- Incremental JSONL scan (path / size / mtime / inode / offset). Truncation or a replaced file is a full rescan of that source
- `wheretoken rebuild` deletes `~/.cache/wheretoken/index.v1.db` and reads agent data again
- Scan stderr names each source `full` / `incremental` / `unchanged`
- `--since 7d`, `--from`, `--to` (local timezone). Dashboard: 今日 / 7 天 / 30 天 / 全部, plus 较上期
- Token accounting and adapter map: `docs/token-accounting.md`. Events carry `derivation`. Observatory JSON has `why`
- `docs/data-sources.md` is the adapter contract; machine snapshots moved to `docs/data-sources/fixtures.md`
- `docs/adding-an-adapter.md` and complementary Claude merge tests (missing fields cannot wipe a sibling column)
- `wheretoken doctor` and `sources` report detect / usage / quality from the same scan
- Dashboard shows 完整 / 降级 / 估算 / 数据不可用 and does not present absent usage as 0.00 M
- README frames whereToken as local-first usage observability: why it exists, how agents are normalized, missing data is not zero
- `verify-cli.sh` looks for the `--offline` banner in the whole table. The 3-line slab is no longer the first two lines

## 0.3.0 — 2026-08-18 (Alpha)

This is an **Alpha**. Please try it, open issues when something is wrong, and send pull requests. macOS GitHub binaries are still unsigned.

- Grok CLI is a tool (`--grok` / `--tool=grok`). Reads `~/.grok/sessions/**/updates.jsonl` `turn_completed.usage`. Vendor stays xAI. Scan is a snapshot — rerun `wheretoken` or 刷新 to refresh today
- Claude Code JSONL without a top-level `requestId` still counts, keyed by `message.id`. Rows with only a per-line `uuid` stay skipped
- After `curl | bash`, the only next line is the installed path
- Accent gold is `#FFD700`. The scan mark is a filled slab with two vertical eye slots; captions stay `挠头中` / `搬煤中` (`--ascii` keeps `(o_o)` / `(^_^)`)
- All-time KPI box adds 当日用量 / 单日最高
- Dashboard home: muted status line, footnotes in 窑口, empty-kiln stage, hit-rate bands on the KPI
- An empty home puts the mark above the zero table: 窑里还是冷的

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
