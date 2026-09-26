# Release acceptance v0.7.7

Prepared 2026-09-26. This is not a published release and not a production verification.

Branch: `cursor/release-v077-3e3c`

PR #9 (`https://github.com/rainhuang0220/whereToken/pull/9`) was still open, draft, `MERGEABLE` / `CLEAN`, head `695bde29c77422d68db744e8992783ce97f5e121`, base `b4c9f81252656c0524208db019771344dfda2658`. CI on that head was green: `test (ubuntu-latest)`, `test (macos-latest)`, and `test (windows-latest)` in run `36227486418`, all `SUCCESS`. The diff against `origin/main` is 24 files, 1545 insertions, 397 deletions. It does not change the 10,000-token or 1-hour thresholds.

`origin/main` is `b4c9f81252656c0524208db019771344dfda2658`. `v0.7.6` still peels to `42f22dcdf85502937e063a8595846d78e35d691d`. The tag object was not moved.

## Completed

- merge SHA: MERGE BLOCKED. `gh` cannot merge, create releases, or push tags. ManagePullRequest (`create_pr` / `update_pr` / `get_ci_status`) is not in this environment. No merge commit was created and none was invented.
- release tag: not published. `v0.7.7` was not created or pushed. `gh release view v0.7.7` returned `release not found`. Latest GitHub Release remains `v0.7.6` (`2026-09-26T05:14:28Z`).
- deployed SHA: not deployed.
- migration result: local disposable tests PASS. Production migration BLOCKED.
- Homebrew result: BLOCKED. The tap was not written and `brew upgrade` was not run.

Version files on this branch:

- `CHANGELOG.md` top release heading is `## 0.7.7 — 2026-09-26 (Alpha)`. Explanation recorded there: unified verified GitHub wall publication state and automatic refresh scheduling.
- `npm/package.json` `version` is `0.7.7`. This is repository metadata. The package is not on the npm registry, and `releases/download/v0.7.7` does not exist, so this must not be treated as a published npm install.
- `Formula/wheretoken.rb` is unchanged at the `v0.7.6` source tarball. A v0.7.7 sha256 cannot be computed until the tag exists. The consistency test allows that one-release lag.
- `cmd/wheretoken/main.go` and `cmd/wheretoken-hosted/main.go` still default `version` to `dev`. Goreleaser stamps the release binary from the tag. No tag was pushed.
- `site/index.html` download label was left version-free.

`go test ./...` PASS and `go vet ./...` PASS on `6224cde71726a7843baf8d985a947e48058bb202`, with `WHERETOKEN_REQUIRE_MYSQL=1` pointed at disposable MariaDB `10.11.14-MariaDB-0ubuntu0.24.04.1` on `127.0.0.1`, database `wheretoken_localtest`. That database and its test user were removed after the run. `cd web && npm test` PASS (172). `bash scripts/verify-cli.sh` PASS. `profile-e2e` was NOT TESTED (`profile-e2e/node_modules` is absent; browsers were not installed). CI does not run on this branch until a pull request exists (`push` is `main` only).

## Local migration

`TestWallPublicationMigrationCases` and `TestBackfillVerifiedIdentityDoesNotInventATotal` PASS. Each case used its own database on the local server (`wt_case_fresh`, `wt_case_v076`, `wt_case_newer`, `wt_case_missing`, `wt_case_failed`). Those databases were dropped by the test. Nothing used a production DSN.

| Case | Result |
| --- | --- |
| Fresh database | Migrate creates nullable `verified_total_tokens` (not default 0), zero presentation rows, no `schema_migrations` table. A second Migrate still has zero rows. |
| Existing v0.7.6-shaped database | Old tables have no verified columns. After Migrate, a README snapshot id that equals the accepted snapshot id and has a JSON total backfills `42000` and date `2026-09-25`. A second Migrate does not change that. Dropping the new columns leaves `snapshot_id` and `total_tokens`. Migrate again restores the same verified identity. |
| Accepted snapshot newer than verified GitHub state | After the proven backfill, replacing the accepted snapshot with a later id and total `80000` does not change verified. Pointing `readme_snapshot_id` at that newer id and migrating again still does not overwrite verified. |
| Missing historical verified watermark | Empty README id, README id that disagrees with the accepted id, and a matching id whose JSON total is null all stay SQL NULL. The null-total row's `total_tokens` column was `0` and was not copied. |
| Pending failed publication | `readme_status=failed` and `readme_last_error=conflict` survive. The migration does not invent `pending_due_at` or a verified total. A later pending due time, reason `usage`, and retry count `2` survive a second Migrate, and verified stays NULL. |

The product has no down migration. Rollback was the explicit `DROP COLUMN` of the eight new columns, then Migrate again. The change is additive. Unknown totals stayed NULL. Accepted was not copied onto an existing verified row.

Production migration: BLOCKED. No production database credentials and no SSH. No backup was taken. No production `Migrate` ran.

This VM already had a MariaDB database named `wheretoken` with the new verified and pending columns present. It was not the test target and not the production server. Its rows were not read and it was not modified.

## Publication verification

CODE TESTED: the publication policy and migration tests in this repo passed locally, including the cases above. PR #9 CI on `695bde2` was green. That is simulation and MariaDB fixture coverage.

REAL PRODUCTION VERIFIED: not observed. No README commit, SVG commit, snapshot id write, or publication timestamp was produced by this release. The public profile read in `engineering/production_before_upgrade.md` is the current live response, not a write. It has no `wall` object. The owner's profile README was not modified. No disposable GitHub repository or publish token was configured.

## Remaining limitations

| Item | Status |
| --- | --- |
| PR #9 merged | BLOCKED. Read-only `gh`. No ManagePullRequest tool. |
| Draft pull request for `cursor/release-v077-3e3c` | BLOCKED. Same missing tool. Branch is pushed. |
| Git tag `v0.7.7` and GitHub Release | BLOCKED. Tag push and release creation are not allowed here, and checksums do not exist yet. |
| In-repo Homebrew formula sha256 bump | BLOCKED until the tag tarball exists. Formula left at v0.7.6. |
| `profile-e2e` Playwright | NOT TESTED. |
| Production SSH / PM2 / env / PID | BLOCKED. Publickey denied for `ubuntu` and `root` at `175.24.134.228`. |
| Production database backup and migration | BLOCKED. No production DSN. |
| Hosted deploy of this commit | BLOCKED. No server write access. `scripts/build-hosted.sh` was not run. |
| Real GitHub wall publication | BLOCKED / NOT TESTED. No disposable repo and no production publish credential. Owner README was not written. |
| Homebrew tap `rainhuang0220/homebrew-wheretoken` | BLOCKED. API permissions `push: false`. Formula on the tap is still `version "0.7.5"`. `brew upgrade` was not run. |
| Local `go test ./...`, `go vet ./...`, web `npm test`, `scripts/verify-cli.sh` | PASS |
| Local wall migration cases on MariaDB 10.11 | PASS |
| Production ready | not ready. Deploy, production migration, and a real wall read-back did not happen. |
