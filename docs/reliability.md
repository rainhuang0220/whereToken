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
- `go test ./internal/cli/ -run 'TestProfileRefreshOutageRetriesAcrossHorizons|TestProfileRefreshRepairDoesNotCallAFailedSlotHealthy|TestDoHTTPRefusesProductionFromATest'`

These are not a production soak. They do not prove the hosted machine, Pages, or a sealed Cursor session.

## Rollout and rollback

Do not merge, deploy, or replace `/opt/homebrew/bin/wheretoken` without a separate approval.

1. Deploy a schema-aware hosted build and verify one legacy snapshot and one candidate snapshot on a staging owner.
2. Repair Pages from the verified mirror and read back manifest, JSON, and both SVGs.
3. Install the candidate CLI only after those two checks. The watch then uses the longer retry. A schedule file with `attempts` greater than 3 makes the 0.7.19 binary exit, and KeepAlive would restart it every minute. Before any rollback, stop the watch and remove `~/.config/wheretoken/profile-refresh-schedule.json`. Leave the ledger in place.
4. Copy `scripts/freshness-alert.sh` and `scripts/freshness_classify.py` into the Mac agent directory only as part of that install. The running copies are not the git tree.

Rollback of the CLI is the previous Homebrew bottle plus the schedule-file step above. Rollback of the host is the previous release directory. Do not delete ledger rows or projection rows to undo a bad publish.
