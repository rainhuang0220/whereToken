# Reliability audit progress

Checkpoint for a new agent. Do not `git reset --hard` or `git clean`. Do not
merge, tag, deploy, or replace the installed CLI without a separate approval.

## Checkpoint

- Updated: 2026-11-18 after the takeover implementation pass
- Workspace: `/Users/rainhuang/Desktop/whereToken`
- Branch: `work/profile-refresh-reliability`
- Starting HEAD: `7d0fcb15c926854ac04e1e0bb07adedea069822d`
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

## Verification

- `63649a1` CI run 44286271381 and upgrade run 44286271385 succeeded on Ubuntu, macOS, and Windows.
- After the follow-up fixes, `go test ./...` passed again, as did the freshness, lifecycle, alert, watchdog, and Pages script tests.
- Isolated hosted tests, including an older snapshot theme retry and an audited correction, passed against the test database. They did not call production.
- Read-only `GET https://wheretoken.plainlist.space/api/health` on this Mac failed. The name resolved to `198.18.0.48` and TLS ended with `SSL_ERROR_SYSCALL`. No response body was read. This is not a schema compatibility check.
- Live watchdog at `2026-11-18T11:12:31Z` wrote `/tmp/wt-watchdog/alerts.json`: hosted unreachable, and Actions last schedule run `2026-11-18T04:19:59Z` so the local class is `check_failed workflow_silent`. No GitHub issue was created.
- The local clock at the check was `2026-11-18 19:11 +0800`. The next natural refresh slot is `00:00`. It was not observed. The installed CLI is still Homebrew 0.7.19 and was not restarted.
- Playwright `profile-e2e` ran 123 tests on Chromium, Firefox, and WebKit. All passed. The server was the local fixture, not production.

## Not done

## Not done

- Hosted watchdog script is in-tree and not installed on the server.
- Gate B stays NO-GO. Background refresh still does not unseal Cursor.
- Theme-retry can still move a verified wall backward. See A09. Not changed.
- Local adapters can still mark a missing directory `FRESH`. See A06. Not changed.
- Quiet local days can still render as empty `0` when no coverage gap is attached. See A07.

## Gate

- Gate A: `GO FOR STAGING / PRODUCTION VERIFICATION PENDING`
- Gate B: `NO-GO`

## Not approved

Merge, tag, Release, Homebrew or npm publish, hosted deploy, production DB
write, replacing `/opt/homebrew/bin/wheretoken`, DNS or proxy edits, Keychain
changes, installing the host cron, new paid infrastructure.
