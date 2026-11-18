# Profile refresh reliability

Enable the macOS user agent with `wheretoken profile refresh on`. Once enabled,
it checks at 00:00, 10:30 and 17:00 in the owner's local time. Linux and Windows
can run `wheretoken profile refresh watch` in the foreground.

```sh
wheretoken profile refresh status
wheretoken profile refresh status --json
wheretoken profile refresh doctor
```

## Collection and publication

The refresh worker reads local usage records and supported account APIs. The
ledger stores known facts and complete account observations. The worker builds a
sanitized snapshot, then checks its totals and coverage against the hosted
projection and the verified GitHub wall. Hosted acceptance and GitHub README
and SVG verification have separate timestamps. The Pages workflow mirrors and
checks the verified bundle independently. A locally generated snapshot can
advance its `as_of_date` while retaining older totals. The activity wall shows
partial coverage and displays uncovered empty Token cells as unknown.

| Evidence | Meaning |
| --- | --- |
| `last_attempt_at` | The last collection attempt for the source |
| `last_successful_observation_at` | The latest trusted account observation, or a successful read of a local source |
| `coverage_through_utc` | The latest fully observed UTC day. Earlier gaps can still exist |
| `gaps_utc` | Inclusive unverified UTC date ranges within the account lookback |
| `snapshot_generated_at`, `as_of_date` | When the snapshot was built and which local calendar date it displays |
| `hosted_accepted_at` | When the hosted API accepted a sanitized snapshot |
| `publication_verified_at` | When the GitHub wall was verified |

Legacy refresh state still loads. It has unknown collection health until the
new worker records evidence. A public snapshot without source evidence displays
unverified collection coverage even when its publication has been verified.
Local log readers can confirm a successful scan
without proving a continuous date range. An empty directory or a quiet calendar
day cannot establish measured zero. A complete usage window with an explicit
zero count or explicit zero-meter rows can report `CONFIRMED_ZERO`. An ambiguous or failed response leaves
coverage unknown. UTC windows and owner-local calendar cells use different day
boundaries, so a gap can affect two displayed dates.

## Cursor authorization and backfill

A readable, account-bound Cursor session can collect unattended. On macOS, the
watcher leaves sealed sessions unopened and reports `AUTH_REQUIRED` when an
account collection needs one. In a foreground terminal, run
`wheretoken cursor connect` to collect with the system's normal authorization
flow. If Cursor's own login is expired, renew it in Cursor first. An upgrade or
reboot can change whether the system grants access. A previous interactive
collection does not enable unattended access to a sealed session.

The project ships no Keychain helper. A stable signature can preserve an
existing grant. The login Keychain must still be unlocked and the provider
session must still be valid. The rollout uses normal foreground authorization
and the no-Keychain boundary in the background. A foreground connection
prompts before acquiring the scan lock and then uses only the exact session it
authorized within that process. Cursor credentials stay in process memory
during a pull. [Apple Keychain APIs](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)

The scheduled reader prioritizes the open UTC month and the previous month
when it needs a refresh. It rotates older gaps through the lookback. An open
day is only observed through the pull and must be revisited after UTC midnight.
The reader accepts a window after pagination and accounting checks. The ledger
commits the window and audit together, keeps stable request identities, and
rejects a drop in already confirmed daily account totals. Repeated windows stay
idempotent. Historical revisions requiring lower values need audited
reconciliation. The public publication guard still protects the independently
verified total. The personal dashboard endpoints are compatibility interfaces
and their history and schemas can change. [Cursor team Admin API](https://prod.cursor.com/docs/account/teams/admin-api)

`wheretoken cursor backfill --from YYYY-MM-DD --to YYYY-MM-DD` enriches model
attribution on existing ledger days. Recover missing totals through account
collection. With a sealed session, repeat the foreground collection as needed.
If upstream cannot supply a window, the gap remains visible and the last
trusted usage stays available.

## Recovery and diagnostics

A healthy watch sleeps between calendar slots. After a transient failure, it
retries after about 30 seconds, 2 minutes, 2 hours, then at four-hour intervals,
with up to 10% jitter. The checkpoint carries the retry deadline across
calendar slots, watch restarts and laptop sleep. A missed slot has one catch-up
check after wake or graphical login. The isolated worker is bounded at five
minutes. The kernel releases its scan lock on exit.

- `STALE` means the observation is at least 24 hours old. Check the scheduler
  and retry deadline. A power-off or absent graphical login delays recovery.
- `AUTH_REQUIRED` needs an interactive Cursor collection or login renewal.
  A scheduled macOS check submits a local notification when the last Cursor
  observation is at least 24 hours old or missing, at most once per 24 hours.
  The checkpoint records the last successful submission as
  `cursor_auth_notice_at`. Banner visibility follows macOS Notification
  settings and Focus. An unrelated publication sign-in failure does not stop
  collection or suppress this reminder.
- `SOURCE_UNAVAILABLE` identifies a provider, local read or ledger failure.
  Doctor and JSON status show fixed categories such as `NETWORK`,
  `RATE_LIMIT`, `SCHEMA`, `LOCAL_READ` and `LEDGER`.
- `COVERAGE_GAP` lists unverified UTC ranges. Known historical gaps rotate
  through the regular slots. A failed provider attempt can also use the retry
  budget. Keep the existing ledger and its backup.
- `RECONCILIATION` indicates a candidate account window that would lower a
  recorded UTC day. Keep the prior observation and investigate upstream
  changes before applying a verified correction.

`status` uses local evidence. `doctor` also reads the ledger and, for the
maintainer Pages target, checks the deployed manifest, JSON and both previews.
Use `profile refresh repair` to check or reload the scheduler. The bounded
schedule log is `profile-refresh-schedule.log` beside the refresh config,
with one 64 KiB rotated file. It records fixed categories, attempts, elapsed
time and the retry deadline. Ordinary reports continue to use the cached path.

## Deployment gates

The reliability patch can ship after hosted compatibility, Pages repair and
the CLI rollout checks pass. A sealed Cursor session continues to require
foreground authorization. Track fully unattended collection on macOS as a
separate requirement.

Deploy the hosted service with the updated optional v2 `source_status` schema
before distributing the CLI. The prior strict v2 validator rejects the new
field. Older snapshots remain valid. The field contains source IDs, fixed
statuses and UTC dates. Exact attempt and observation times stay on the device.
The activity wall still uses 371 fixed date positions.

After upgrading from v0.7.19, run `wheretoken profile refresh on` with the new
binary if automatic refresh was already enabled. This installs the new
launchd logging configuration. The ledger tables already exist. Existing
observations remain readable and an offline backup can be restored with the
watch stopped if a rollback is needed. On the Pages side, the scheduled check
can dispatch repair for a missing or stale deployment even when the mirror
commit is already present. The browser uses a verified hosted snapshot when
its date and cumulative total preserve the accepted baseline, and keeps the
static bundle available when the hosted API cannot be reached.


### Hosted-first compatibility gate

In staging, submit an ordinary v2 snapshot from the old CLI, then submit a
candidate v2 snapshot carrying `source_status` for the same test owner and a
later local date. Keep the cumulative total monotonic. Require the hosted PUT
and subsequent GET to agree on snapshot ID and date. Compare the returned
source evidence with the candidate and verify the ID on both README previews.
Only then allow the candidate CLI to upload. The hosted integration test
covers both payloads and rejects an attempt to reuse the candidate ID after
stripping its source metadata.

For rollback after the first candidate write, keep a schema-aware hosted
reader in service and stop rolling out the candidate CLI or pause the test
watch. Stored candidate revisions include source evidence in the ID. Keep the
existing live projection, verified wall and backup intact.

### Patch rollout

1. Build the hosted artifact from the approved commit with
   `sh scripts/build-hosted.sh <artifact-dir>`. Record its build revision with
   `go version -m <artifact-dir>/wheretoken-hosted`. On the authorized host,
   preserve the current binary, SPA, environment and a consistent MySQL backup.
   Follow the [hosted deploy runbook](../engineering/v0.7.7_operator_runbook.md)
   for paths and process checks. After deployment, check `/api/health` and
   repeat both snapshot readbacks with the staging owner before enabling CLI
   uploads. Use a schema-aware hosted artifact for rollback once candidate
   snapshots have been accepted.
2. Point the freshness workflow in the staging Pages repository at its own
   hosted owner and Pages URL. Leave the expected mirror committed while
   Pages serves the previous revision. Dispatch `profile-freshness.yml` in
   that repository and record the main commit before and after. Wait for
   `pages.yml` and run `python3 scripts/profile_pages.py --dir public-profile
   --url <pages-profile-url> --attempts 5`. Require the committed snapshot
   ID and `pages_mirror=verified`. Compare that ID with the verified hosted
   wall and repeat the readback after the approved rollout.
   [GitHub runner labels](https://docs.github.com/en/actions/reference/runners/github-hosted-runners)
3. Run `bash scripts/candidate-upgrade.sh` to exercise the published v0.7.19
   updater against a local candidate. For the macOS pilot, run the candidate
   by its absolute path and compare `profile refresh status --json` with
   `profile refresh doctor --json`. An old CLI can load the same v1 state
   but reports its own scheduler health, so use the candidate to judge
   collection. Keep the ledger and its backup during the trial.
4. After the hosted, Pages and CLI checks and release approval, use the
   [patch release checklist](releasing.md) for v0.7.20. Push the release tag
   only when ready to publish. Check the release artifact checksums before
   updating the Homebrew and npm version references.

### macOS pilot over two scheduled slots

Use a separate macOS GUI test account, its own ledger, a test Cursor account
and a staging publish target. Record the candidate binary digest, signature
status, owner time zone, each surface's snapshot ID, and the ledger's last
Cursor observation and coverage. The pilot operator owns any real account
login, Keychain prompt, scheduler setup and external publication.

1. **First slot at 10:30 local.** Create measurable usage after the baseline
   in a supported local source and let the candidate watch run under launchd.
   Verify the new ledger facts and the public readback. A sealed Cursor
   session should report `AUTH_REQUIRED` when its account coverage cannot
   advance. For the separate unattended test, generate usage in the test
   Cursor account. Require a new trusted API observation, account continuity
   and a complete audit without an operator, prompt or plaintext credential
   storage. Record any foreground collection separately.
2. **Second slot at 17:00 local.** Before the slot, have the staging operator
   introduce a reversible provider outage or use an expired test session.
   Require the collection to report degradation, retain the baseline
   observations and avoid zeros for unobserved dates. Restore the staging
   provider or renew the test session. Let the bounded retry run or use
   the next scheduled check. Require new verified coverage and match the
   accepted, verified and displayed snapshot IDs. An auth-required case
   with stale or missing evidence should submit one local notification.
   Verify the banner in the test GUI and repeat a failed check within
   24 hours to confirm cooldown persists across a worker restart.
3. **Check Pages separately.** Leave the staging Git mirror bytes unchanged
   while the site serves an older bundle. Require the scheduled job to
   request deployment and the next read-back to match the committed
   manifest, JSON and preview SVGs. Confirm a browser that already saw
   newer verified data keeps it on reload in the same tab during a hosted
   failure.

### Signed Bridge decision

The earlier macOS isolation test exercised a disposable Keychain item:
stable signing and a fixed designated requirement survived a rebuild. The
separately approved one-time Cursor prototype read succeeded, and a later
ACL audit recorded unexpected persistent trust entries. Those results establish
that signature continuity and per-item authorization are separate controls.
The installed v0.7.19 binary inspected for this review has an ad-hoc signature.
The former prototype is outside the deployment plan. A new pilot would require
an exact reviewed signing identity, a fresh authorization scope, and before/after
access-control evidence from a disposable GUI account.

The current release gate for sealed-session unattended acquisition remains
**NO-GO**. The local reminder and foreground acquisition provide a fallback.
A genuinely unattended design needs a provider-supported scoped credential
or a separately accepted OS authorization with documented revocation,
signed-upgrade continuity and repeatable two-slot evidence.
