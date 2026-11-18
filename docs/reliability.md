# Reliability

whereToken stays useful when a network, a provider, or a publisher fails.
A failure must stay visible, a retry must be bounded, and a success on one
step must not erase a failure on another.

## Separate results

| Checkpoint | Meaning |
| --- | --- |
| Scheduler process | The macOS watch is running |
| `last_attempt_at` / refresh `code` | The last collection attempt and why it stopped |
| Ledger commit | Confirmed facts and audited account windows |
| `hosted_accepted_at` | The hosted API stored a sanitized snapshot |
| `publication_verified_at` | README and both SVGs matched after publish |
| Pages repair | The deployed Pages bundle matched the verified mirror |
| Browser snapshot | The tab kept a newer verified profile |

`healthy=yes` requires the data checkpoint, not a live PID. `profile refresh repair` prints `scheduler_healthy`, `data_healthy`, and `healthy` separately. The hourly agent writes `~/Library/Logs/wheretoken/profile-refresh-health.log`.

## Incidents

One open GitHub issue per surface. Titles are exact:

- `profile-freshness alert (actions)` — public wall and public source status
- `profile-freshness alert (local)` — this Mac's checkpoint and the Actions heartbeat
- `profile-freshness alert (pages)` — Pages repair

Recovery closes only that title. A class change edits the body. A failed issue lookup prints `class=check_failed lookup-failed` and does not print `class=ok`. Legacy dated titles are left alone.

The Mac agent may dispatch `profile-freshness.yml` at most once every three hours when it sees `workflow_silent`. Actions does not dispatch itself.

## What watches what

| Failure | Who notices |
| --- | --- |
| Mac offline, wall older than 36h | GitHub Actions `wall_stale` |
| Refresh failing while the Mac is up | Local issue, from `WILL_RETRY` or a blocked code, without waiting for the wall |
| Hosted API down | Actions and the Mac, as `check_failed` |
| Actions not running | The Mac, as `workflow_silent`, including when the wall is already stale |
| Pages behind the verified mirror | The Pages issue. A fresh wall does not close it |
| Only Cursor needs authorization | `collection_stale` / `AUTH_REQUIRED`. Other sources still collect |
| A real zero-usage day | `CONFIRMED_ZERO` is healthy. Missing evidence is not zero |

A second job in the same Actions workflow is not an independent watchdog. `scripts/hosted_freshness_watchdog.py` is the third path, for a cron on the hosted machine. It is not installed. Until that cron exists, Actions-down detection depends on the Mac being awake.

## Data rules

A proven wall is not replaced by an earlier `as_of_date` or a lower total.
`publishPalette` checks `VerifiedWallRegression` before any GitHub write, including theme retry and job retry. A lower total is published only when the request context carries an audited correction whose delta matches that drop. A stored `corr:` tag is not a permit.

A source with no install is omitted. A directory that is not there is `ABSENT`. A read error is `LOCAL_READ` and `SOURCE_UNAVAILABLE`. A directory that reads and yields no events is `NO_EVENTS` and `UNKNOWN`. Only a read that returned events is `FRESH`, and that still does not prove a quiet day was zero.

A token or request cell is `empty` only when every relevant source is `CONFIRMED_ZERO` through that date. Every other quiet cell is `unknown`. A period whose events sum to zero and is not an audited zero is unavailable, not `0`.

## Watchdog

`scripts/hosted_freshness_watchdog.py` fetches the public profile and the profile-freshness Actions runs, writes a state file, and appends one alert record per surface. `--alert-command` is the optional delivery into `freshness-alert.sh`. The script does not open a GitHub issue unless that command is set.

On this Mac at `2026-11-18T11:12:31Z` the public profile fetch failed. System resolution of `wheretoken.plainlist.space` was `198.18.0.48` and `2001:2::2f`, and TLS ended with `SSL_ERROR_SYSCALL`. Actions list succeeded. The last schedule run was `2026-11-18T04:19:59Z`. The alert file recorded `check_failed` and `check_failed workflow_silent`. No GitHub issue was opened.

A cron on the hosted machine, plus `--alert-command` pointed at the alert script, is not installed. Until that cron runs, this Mac cannot confirm the origin is up, because its default resolver does not reach it.

## Retry

The candidate watch keeps a deadline across sleep, restart, and the next calendar slot: about 30s, 2m, 2h, then 4h, with jitter. It does not stop after three attempts. The installed 0.7.19 binary does stop, which is why a slot can sit at `WILL_RETRY` until the next clock time while repair still logs healthy.

Auth and other definitive client errors are not retried. A refresh PUT that cannot be confirmed is retried later; a confirm GET that already shows the snapshot id does not PUT again.

Cursor billing treats a full page with no next cursor as an incomplete window. It does not store that page as covered days.

## Cursor on macOS

Gate B is NO-GO. The background watch does not unseal a Keychain session. The recovery path is `wheretoken cursor connect` in a foreground terminal, then an idempotent backfill that will not attach history to the wrong account. Do not add a keychain helper or widen an ACL.

## Tests that must keep passing

- `scripts/freshness_classify_test.py` — a fresh wall does not hide `WILL_RETRY` or an Actions gap
- `scripts/freshness_alert_test.sh` — surfaces do not close each other; a bad issue list is not `class=ok`
- `scripts/freshness_lifecycle_test.py` — 7, 14, and 30 virtual days
- `scripts/freshness_watchdog_test.py` — wall down, Actions silent, Mac offline, and a state path the script cannot write
- `go test ./internal/cli/ -run 'TestProfileRefreshOutageRetriesAcrossHorizons|TestProfileRefreshRepairDoesNotCallAFailedSlotHealthy|TestDoHTTPRefusesProductionFromATest|TestScheduleDowngradeKeepsEvidenceAndStaysRunnable'`
- `go test ./internal/hosted/ -run 'TestThemeRetryDoesNotReplaceANewerVerifiedSnapshot|TestAuditedCorrectionMayLowerTheVerifiedTotal'`
- `go test ./internal/publicprofile/ -run 'TestQuietDayWithoutCoverageIsUnknown|TestConfirmedZeroDayStaysZero'`
- `go test ./internal/scan/ -run 'TestMinimaxDirectoryWithoutLedgerIsNotFresh'`

These are not a production soak. They do not prove the hosted machine, Pages, or a sealed Cursor session.

## Rollout and rollback

Do not merge, deploy, or replace `/opt/homebrew/bin/wheretoken` without a separate approval.

1. Deploy a schema-aware hosted build. On an isolated database, PUT one legacy snapshot and one candidate snapshot and read both back. A production check is a separate read-only GET. A failed GET is not a schema check, and an isolated PUT is not a production write.
2. Repair Pages from the verified mirror and read back manifest, JSON, and both SVGs.
3. Install the candidate CLI only after those two checks. The watch then uses the longer retry. The schedule file keeps `attempts` at 3 or less. The outage count lives in `failures` and in `profile-refresh-evidence.json`.
4. Copy `scripts/freshness-alert.sh` and `scripts/freshness_classify.py` into the Mac agent directory only as part of that install. The running copies are not the git tree.

Downgrade to v0.7.19 while the candidate binary is still the one you run:

```sh
wheretoken profile refresh prepare-downgrade
```

That copies the schedule to `profile-refresh-schedule.json.pre-downgrade`, refreshes the evidence file, and rewrites `attempts` to at most 3. Then install the 0.7.19 bottle. v0.7.19 can load that file, so KeepAlive does not restart a rejected schedule every minute. Its next save drops fields it does not know. The evidence file and the pre-downgrade copy stay. The ledger is not opened. Do not delete the schedule file to roll back. If `attempts` is already above 3 and the 0.7.19 binary is installed first, stop the watch, run the candidate `prepare-downgrade` against the same home, then start 0.7.19.

Rollback of the host is the previous release directory. Do not delete ledger rows or projection rows to undo a bad publish.
