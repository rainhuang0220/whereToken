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

## Verification before this commit

- `go test ./...` passed.
- `go vet ./...` passed.
- `python3 scripts/freshness_classify_test.py`, `scripts/freshness_lifecycle_test.py`, and `bash scripts/freshness_alert_test.sh` passed.
- Not run: Playwright, a live Mac slot, a hosted deploy, or a production GET after the Fake-IP EOF.

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
