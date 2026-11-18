# Reliability audit progress

Checkpoint for a new agent. Do not `git reset --hard` or `git clean`. Do not
merge, tag, deploy, or replace the installed CLI without a separate approval.

## Checkpoint

- Updated: 2026-11-18 after CI on `0388d3e`
- Workspace: `/Users/rainhuang/Desktop/whereToken`
- Branch: `work/profile-refresh-reliability`
- Starting HEAD: `7d0fcb15c926854ac04e1e0bb07adedea069822d`
- Code gate HEAD: `0388d3e771bb11d57a2c0117c65b02a30e17c660`
- Draft PR: https://github.com/rainhuang0220/whereToken/pull/60
- Working tree at start had no tracked edits. Untracked and left alone:
  `.superpowers/`, `docs/superpowers/plans/2026-09-01-v060-model-pricing-portrait.md`,
  `image.png`
- Audit notes: `/tmp/wheretoken-audit/A01.md` through `A15.md` as each agent finished
- Design note: [`docs/reliability.md`](reliability.md)

## Production, read only

- Installed CLI remains Homebrew `wheretoken 0.7.19`. It was not replaced.
- The 17:00 slot on 2026-11-18 was `phase=failed`, `code=WILL_RETRY`, attempts 3.
  v0.7.19 does not retry that slot again until the next calendar time.
  The health log said `healthy=yes` because repair only checked the PID.
- `/etc/resolver/wheretoken.plainlist.space` is `nameserver 223.5.5.5`. It was
  not edited. A public GET during audit resolved `198.18.0.48` and hit TLS EOF.
  That shows Fake-IP on the default path. It does not by itself prove the
  scoped resolver is the outage, and it was not changed.
- Ledger and `backups/ledger-pre-refresh-reliability-20261117.sqlite3` were not modified.

## What this pass changed

- Repair reports scheduler health and data health separately and exits non-zero
  when the checkpoint is failed past the short retry budget. It does not bootout
  a running watch. The health plist logs to the health log again.
- Freshness issues are one per surface. A recovery does not close another
  surface. Class changes update the body. A bad `gh` list is not `class=ok`.
  Pages repair has its own class. The Mac path can dispatch the Actions
  workflow at most once every three hours when Actions looks silent.
- Tests refuse to dial `wheretoken.plainlist.space`. `scripts/verify-local.sh`
  scans with `--offline` and a loopback base.
- `doHTTP` has a two-minute timeout instead of `http.DefaultClient`.
- A full Cursor billing page with an empty next cursor is an incomplete window.
- The profile page rejects an older generation and a far-future stamp, and a
  trend that includes an unknown day is labeled partial instead of summing it
  as zero.
- Virtual-clock coverage: 7, 14, and 30 days of retry, and the same horizons
  for the incident lifecycle.

## Follow-up after 63649a1

`0d401cc` closes the three data holes. `0388d3e` creates the downgrade
schedule directory with `filepath.Dir`, so the Windows test can open the file.

- `publishPalette` calls `VerifiedWallRegression` before any GitHub write,
  including theme retry. An earlier verified date is refused. A lower total
  is published only when the request context carries an audited correction
  whose delta matches that drop. A stored `corr:` tag is not a permit.
- A source with no install is omitted. A missing directory is `ABSENT` and
  `UNKNOWN`. A read error is `LOCAL_READ` and `SOURCE_UNAVAILABLE`. A
  directory that reads and yields no events is `NO_EVENTS` and `UNKNOWN`.
  An empty scan is not `FRESH`.
- A token or request cell is `empty` only when every relevant source is
  `CONFIRMED_ZERO` through that date. Every other quiet cell is `unknown`.
  A period that sums to zero without that evidence is unavailable.
- `profile refresh prepare-downgrade` copies the schedule to
  `profile-refresh-schedule.json.pre-downgrade`, clamps `attempts` to at
  most 3, and refreshes `profile-refresh-evidence.json`. It does not open
  the ledger and it does not bootout launchd.
- `scripts/hosted_freshness_watchdog.py` fetches the public wall and the
  profile-freshness Actions runs, writes a state file, and appends one
  alert record per surface. `--alert-command` is the delivery hook. The
  host cron is not installed.

## Verification

- `63649a1` CI run 44286271381 and upgrade run 44286271385 succeeded on Ubuntu, macOS, and Windows.
- `0d401cc` CI run 44292999817 failed on Windows only. `TestScheduleDowngradeKeepsEvidenceAndStaysRunnable` could not create the schedule parent because the test split the path on `/`. Upgrade run 44292997979 succeeded on Windows, macOS, and Ubuntu.
- `0388d3e` CI run [44294355935](https://github.com/rainhuang0220/whereToken/actions/runs/44294355935) succeeded. Windows finished in 3m48s, macOS in 1m36s, Ubuntu in 22m03s. The Linux step `Public profile browser tests` passed from 11:26:22Z to 11:45:56Z (`npm ci`, Playwright install, `npm test` in `profile-e2e`). govulncheck passed. Upgrade run [44294356139](https://github.com/rainhuang0220/whereToken/actions/runs/44294356139) succeeded on Windows, macOS, and Ubuntu. `scripts/candidate-upgrade.sh` sets `WHERETOKEN_HOME` to an empty temp directory.
- Isolated hosted Go tests put an older snapshot through theme retry and an audited lower total, then read the stored wall back. The database is the test database. Those tests did not call production.
- Read-only `GET https://wheretoken.plainlist.space/api/health` on this Mac failed. The name resolved to `198.18.0.48` and TLS ended with `SSL_ERROR_SYSCALL`. No response body was read. This is not a schema compatibility check.
- Live watchdog at `2026-11-18T11:12:31Z` wrote `/tmp/wt-watchdog/alerts.json` with classes `check_failed` and `check_failed workflow_silent`. The last profile-freshness schedule run was [44236582881](https://github.com/rainhuang0220/whereToken/actions/runs/44236582881) at `2026-11-18T04:19:59Z`. No GitHub issue was opened.
- A local Playwright run before the push passed 123 tests on Chromium, Firefox, and WebKit. The server was the local fixture. The CI Linux step above is the remote rerun of that suite.
- The local clock at the slot check was `2026-11-18 19:11 +0800`. The next natural refresh slot was `00:00`. It was not observed. The installed CLI is still Homebrew 0.7.19 and was not restarted.

## Not done

- The watchdog script is in the tree and is not installed as a host cron. The live alert file is on this Mac. A production GitHub issue was not opened.
- This Mac did not receive a production health body, so production schema compatibility is still unchecked.
- No natural 00:00, 10:30, or 17:00 slot has been observed on the candidate binary.
- Gate B stays NO-GO. Background refresh still does not unseal Cursor.

## Gate

- Gate A: `GO FOR STAGING / PRODUCTION VERIFICATION PENDING`
- Gate B: `NO-GO`

## Not approved

Merge, tag, Release, Homebrew or npm publish, hosted deploy, production DB
write, replacing `/opt/homebrew/bin/wheretoken`, DNS or proxy edits, Keychain
changes, installing the host cron, new paid infrastructure.
