# Performance

This note explains why interactive whereToken reports were slow, which changes actually moved the cost, and what is still expensive. It is an engineering reconstruction from the changelog, the current call path, and measurements on one machine. Sentences marked as engineering analysis are not quotes from a commit message.

Speeding up a query is not the same thing as generating newer usage. A cached report does not call provider APIs and does not mean the numbers are the live cloud total.

## What used to be slow

The original report path is still `App.doScan` → `scan.Run`. Every adapter discovers its files and parses them. For Cursor that opens `state.vscdb` and reads `cursorDiskKV` rows whose keys start with `composerData:` and `bubbleId:`. On a machine whose Cursor database has grown for months, that parse dominates a cold report. The product also has to deduplicate by request id, keep account-bound history separate from later accounts, and refuse to show missing usage as zero. A cache that skipped those rules would have been a faster wrong answer.

Engineering analysis: a full scan was a reasonable first design while each tool's local log was small, while the report was the thing that collected data, and while there was no second process allowed to refresh that data. Those assumptions stop holding once one SQLite file is multi-gigabyte and a person runs the CLI many times a day.

## v0.7.13 — query shape, not a cache

v0.7.13 added `wheretoken YYYY-MM-DD` and vendor attribution for Cursor Composer. The changelog for that version says the scheduler, publishing policy, and token accounting did not change. It is not a performance release. Date filters run after events are loaded; they do not avoid reading the sources.

## v0.7.14 — two different changes

### Indexed Cursor reads

`internal/adapter/cursor/cursor.go` documents the mechanism. Cursor's `key` column uses BINARY collation. SQLite's default `LIKE` is case-insensitive, so a prefix `LIKE 'composerData:%'` cannot use that index and walks the table. A case-sensitive `GLOB 'composerData:*'` (and the same for `bubbleId:*`) is an index range. Tests lock the query text. The changelog says this helps a full scan and the interactive fallback, and it explicitly does not claim that an 11 GB database becomes instant.

This is an algorithm/plan change for the scan path. It does not change how often a scan runs.

### Read the normalized index instead of scanning

`internal/cli/report_cache.go` `cachedReport` is the interactive path when all of the following hold:

- the process is macOS
- Profile auto-refresh is on
- `App.Scan` is nil and `WHERETOKEN_NO_INDEX` is unset
- `index.OpenReadOnly` succeeds
- `ExportCached` returns events and zero integrity gaps

The index stores per-source checkpoints from the last successful scan. It is not one atomic snapshot of every tool, which is why the report prints `缓存报告 · 本地索引数据（扫描时可能分批更新），非实时`.

Confirmed Cursor account history is not inside that file index alone. `ledger.OpenReadOnly` appends ledger rows the index has not seen (`Ledger.Unseen`). The open is `mode=ro`. It does not migrate or write. If the index or the ledger cannot be read, or the export has gaps, `cachedReport` returns not-cached and `runReport` falls through to `doScan`. A broken cache must not look like an empty week.

`wheretoken scan` still parses sources and is what fills the index. `wheretoken rebuild` deletes the index and reads the tools again. Interactive `wheretoken` does not.

This is a caching and scheduling split: collection and reading are different processes. The read is faster because it does not open Cursor's `state.vscdb`. It is staler by design. The background watcher is what is allowed to scan.

### Fewer scans, not a faster scan

`internal/publicprofile/schedule.go` `fixedRefreshSlots` is 00:00, 10:30, and 17:00 in the owner's local time, plus one catch-up if a slot was missed. The watcher sleeps between slots. A repeated wake inside a slot that already finished does not scan again.

That change does not make one scan faster. It cuts how often the expensive scan, the SQLite reads, the filesystem walk, and the profile network publish run. Idle CPU between slots is the watcher sleeping, which is a different claim from "the report got faster."

v0.7.14 also states the publish thresholds (1 token under 1M, 0.01M from 1M, 0.01B from 1B). Those are publication gates, not query performance.

## v0.7.15 — stop background Keychain waits

Unsigned builds can block inside macOS `SecItemCopyMatching` even when the caller asked not to show UI. A scheduled refresh then sits for minutes. `prohibitCursorOSKeychain` on the unattended path refuses OS Keychain unsealing for a sealed Cursor session. Local Cursor activity and other tools still scan. Previously verified account history is reused only when that account has a recorded consent. `wheretoken cursor connect` and a manual `wheretoken scan` still use the interactive authorization path.

This removes a multi-minute stall from the background job. It is not a faster interactive cache read. An owner who has not connected Cursor still does not get invented account tokens.

## v0.7.16 — price reports use the same cache

`runPricingUsage` calls `cachedReport` before `doScan`, same as `runReport`. Before that, `wheretoken` could be a cache read while `wheretoken pricing --usage` parsed every source again, including Cursor. The price table itself did not get cheaper; it stopped repeating the scan.

## v0.7.18 — model identity, not speed

v0.7.18 keeps a measured model id when no public price card matches, and it stops a shorter `--model` query from summing a different unpriced id. Cost is still `price.Event` per event. That work is correctness. It does not remove a scan or an index lookup.

## Why the cache waited until v0.7.14

Engineering analysis, tied to the code that exists now:

- Token totals, request dedupe, and account isolation had to be true before a second copy of those totals was worth reading.
- The durable ledger and the scan index are the copies. Until those existed and were readable without migrations (`OpenReadOnly`) and without hiding gaps (`ExportCached` gap count), a cache would have been able to show a partial scan as a complete one.
- The macOS refresh switch is the signal that some other process is allowed to collect. Without that collector, every interactive command still had to be the collector.
- Once the watcher owns collection, the CLI can be a reader. That is the split.

No commit message records a rejected design. This paragraph is not a claim about what anyone thought at the time.

## What a cached report still spends time on

`cachedReport` calls `metric.Aggregate` before it returns. `runReport` then calls `report.Build`, which aggregates the same events again for the table, calendar, and portrait. Both stay on the interactive path. The resident set is the decoded event slice, not a few index pages.

The mascot on a cached report is the same `scanHUD` sprite used during a full scan. `beginWait` arms it 140ms after the command starts. If the report finishes first, nothing is drawn. Otherwise the ticker repaints every 120ms with the caption `正在整理用量…` and no fake percentage. `finish` erases those rows before the table or JSON is written, so cursor-up cannot eat the result. `--json`, `--quiet`, a non-TTY stdout (a pipe), and `TERM=dumb` do not arm it. A full scan still uses the per-source captions (`正在读 …`) and is not replaced by the generic caption.

`wheretoken scan` remains the command that parses sources. A cached report that misses the index falls back to that scan. The cache is not the live cloud total.

## Measurements

Machine: Apple M4, 24 GiB, Darwin 25.5.0 arm64, local ledger, network not used by these commands. Binary under test for this table: Homebrew `wheretoken 0.7.18`. Command family: cached report (`缓存报告` is in the output). The production Cursor database was not copied and `wheretoken scan` was not run for this table.

Each figure is 7 back-to-back runs. Median is the 4th sorted sample. With 7 samples the nearest-rank 95th percentile is the maximum. `time -l` RSS and CPU are a second series of 7 `wheretoken --quiet` runs, not the TTFB series.

| Path | First stdout byte | Total | User CPU | Max RSS | Scans Cursor? |
|---|---:|---:|---:|---:|---|
| v0.7.18 `wheretoken --quiet` | 4583 ms median, max 5763 ms | same as first byte | 4.98 s median user, 4.58 s real | 1.18 GiB median | no (cache hit) |
| v0.7.18 `--json --quiet` | 4734 ms median, max 8478 ms | same as first byte | not separately sampled | not separately sampled | no |
| v0.7.18 `wheretoken 2026-11-09 --quiet` | 2571 ms median, max 2696 ms | same as first byte | not separately sampled | not separately sampled | no |
| Full scan of this machine's Cursor DB | 未测 | 未测 | 未测 | 未测 | would scan; not run |
| Cached report with the mascot, interactive TTY, `TERM=xterm-256color` | 173 ms median first byte (5 runs: 166, 169, 173, 174, 192) | 5401 ms median total (max 8105 ms) | not sampled separately; quiet non-TTY of the same binary is below | not sampled | no |
| Same binary, `--quiet`, stdout not a TTY | 4802 ms median total, which is also the first byte | 4802 ms | not sampled | not sampled | no |

The mascot runs are a PTY with both stdout and stderr attached, so the first byte is the sprite, not the table. Those five runs painted the caption 36, 43, 44, 65, and 66 times, then printed the report. `--quiet` on the same binary, with no TTY, stays in the same multi-second band as v0.7.18. The mascot does not remove the aggregate. It moves the first visible byte from the end of the run to about 140ms plus scheduling.

These two series were not paired against the v0.7.18 series on a quiet machine. Do not subtract 4583 from 5401 and call the difference the cost of the sprite.

First stdout byte equals total time on v0.7.18 because `--quiet` prints nothing until aggregation finishes. That is the blank wait. A single idle sample earlier the same night, not part of this 7-run series, printed a human report in 2.27 s. The 7-run series is slower. Treat them as different conditions: the series allocates about 1.2 GiB, seven times, back to back. During the series the 17:00 local refresh slot was also due. After the series the Cursor total was still 9,136,507,911 and the unallocated line was still 1,184,162,802. The ledger file's size changed; the checked token anchors did not.

Do not read the missing full-scan row as "the cache is N times faster than parsing." Those are different operations. The cache avoids the parse. It does not make the parse that `wheretoken scan` still does any cheaper except for the v0.7.14 `GLOB` plan, which was not re-benchmarked here.

## Paired check on the release candidate

Same Apple M4, same local ledger, same afternoon. Five runs of the candidate binary built from `1cfc2b2`, through a PTY with `TERM=xterm-256color`. Quiet runs are the same binary with `--quiet` (no mascot). Median is the middle of five. P95 is the highest of five. Cursor's reported total stayed 9,136,507,911.

| Path | First byte median / P95 | Total median / P95 | Mascot frames | Scans Cursor? |
|---|---|---|---|---|
| `wheretoken` | 161 ms / 196 ms | 4564 ms / 5246 ms | 36–42 | no |
| `wheretoken --quiet` | 4538 ms / 4864 ms | 4570 ms / 4890 ms | 0 | no |
| `wheretoken 2026-11-09` | 161 ms / 161 ms | 2655 ms / 2747 ms | 20–22 | no |
| `wheretoken 2026-11-09 --quiet` | 2808 ms / 2944 ms | 2853 ms / 2980 ms | 0 | no |
| `wheretoken pricing --usage` | 161 ms / 165 ms | 4307 ms / 4434 ms | 34–36 | no |
| `wheretoken pricing --usage --quiet` | 4247 ms / 4302 ms | 4271 ms / 4338 ms | 0 | no |

The animated default total (4564 ms) matches the quiet total (4570 ms). The sprite changes when the first byte appears. It does not shorten the aggregate. CPU and RSS were not sampled again in this pair; the earlier v0.7.18 `--quiet` series remains the RSS figure (1.18 GiB median).

## Remaining bottlenecks

- Two aggregates per cached report (`cachedReport` and `report.Build`).
- The event slice is large enough to push RSS near 1.2 GiB, so the cost is allocation and the portrait/calendar pass, not index I/O alone.
- A cache miss, `WHERETOKEN_NO_INDEX=1`, refresh switched off, or a gapped index still takes the full scan, including Cursor `state.vscdb` when that file is present.
- `wheretoken scan` and the three daily refresh slots still do that scan. Fewer times, not zero times.
- The mascot does not reduce any of the above. It only fills the quiet interval on an interactive terminal.
