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
new worker records evidence. Local log readers can confirm a successful scan
without proving a continuous date range. An empty directory or a quiet calendar
day cannot establish measured zero. A complete usage window with an explicit
zero count can report `CONFIRMED_ZERO`. An ambiguous or failed response leaves
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
existing grant, but cannot guarantee that a login Keychain is unlocked or that
a provider session is available. The rollout uses normal foreground
authorization and the existing no-Keychain boundary in the background. Cursor
credentials stay in process memory during a pull. [Apple Keychain APIs](https://developer.apple.com/documentation/technotes/tn3137-on-mac-keychains)

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
  An unrelated publication sign-in failure does not stop collection.
- `SOURCE_UNAVAILABLE` identifies a provider, local read or ledger failure.
  Doctor and JSON status show fixed categories such as `NETWORK`,
  `RATE_LIMIT`, `SCHEMA`, `LOCAL_READ` and `LEDGER`.
- `COVERAGE_GAP` lists unverified UTC ranges. Retry with the same account and
  keep the existing ledger and its backup.
- `RECONCILIATION` indicates a candidate account window that would lower a
  recorded UTC day. Keep the prior observation and investigate upstream
  changes before applying a verified correction.

`status` uses local evidence. `doctor` also reads the ledger and, for the
maintainer Pages target, checks the deployed manifest, JSON and both previews.
Use `profile refresh repair` to check or reload the scheduler. The bounded
schedule log is `profile-refresh-schedule.log` beside the refresh config,
with one 64 KiB rotated file. It records fixed categories, attempts, elapsed
time and the retry deadline. Ordinary reports continue to use the cached path.

## Deployment

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
