# Release acceptance v0.7.7

Observed 2026-09-26 17:41–17:50 UTC. Version choice, merge, and production activation were re-checked in this session.

## Version

0.7.7 stays the patch. The publication watermark moved from "hosted accept" to "README and SVG pair that were read back and matched." That is a behavior change. `AGENTS.md` and `docs/releasing.md` still say ship 0.7.x patches and do not bump minor or major unless asked. The release skill says the same. Those rules agree, so 0.8.0 was not tagged and no minor-bump conflict is open.

Already prepared on this branch, and left as-is except the changelog sentence that states that choice:

- `CHANGELOG.md` heading `## 0.7.7 — 2026-09-26 (Alpha)`.
- `npm/package.json` version `0.7.7`. The package is not a published npm install. `releases/download/v0.7.7` does not exist.
- `cmd/wheretoken/main.go` and `cmd/wheretoken-hosted/main.go` still use `version = "dev"`; goreleaser stamps the tag.
- `Formula/wheretoken.rb` still points at the v0.7.6 source tarball, sha256 `384780534bf6051ca546519ac74182d6f6bfb6331677c04299030a18399417bf`. No v0.7.7 tarball exists, so no checksum was invented.

Thresholds in the changelog remain 10,000 raw tokens and one hour. No scheduler was added.

## Deployment

- old commit (running host, from the health version): `9af013644f5caf52d3eb056886ad7b1e1894266c`. Health string `v0.7.5-0.20260923181126-9af013644f5c`. `git describe` is `v0.7.4-7-g9af0136`. This is not the v0.7.6 release.
- new commit: not deployed. Intended landing branch is `cursor/release-v077-3e3c`. Policy commit `695bde29c77422d68db744e8992783ce97f5e121` is an ancestor of that branch. `origin/main` is still `b4c9f81252656c0524208db019771344dfda2658`.
- migration result: NOT RUN on production. No backup was taken. SSH to `ubuntu@175.24.134.228` and `root@175.24.134.228` returned `Permission denied (publickey)`. No private key, no agent identities, no production DSN.
- rollback point: the running binary identified by that health string and commit. The binary file, process id, startup command, and database dump were not captured. `v0.7.6` was not moved and is not that running binary. Details: `engineering/PRODUCTION_BASELINE.md`.

MERGE BLOCKED. Pull request #10 (`https://github.com/rainhuang0220/whereToken/pull/10`) is an unmerged superset of pull request #9 (`https://github.com/rainhuang0220/whereToken/pull/9`). #9 head `695bde29c77422d68db744e8992783ce97f5e121` is an ancestor of #10. #10 adds only the v0.7.7 prep and engineering notes. Merging both would land the policy twice or conflict. The single landing is merging #10. #9 stays open until that merge exists.

No merge tool was available. `gh` is read-only here. `gh api repos/rainhuang0220/whereToken` reports `push: false` (also `admin`, `maintain`, `pull`, and `triage` false). `gh api user` returned HTTP 403 `Resource not accessible by integration`. Pushing `main` to imitate a merge was not done. No merge SHA.

CI on #10 head `5d74b2ab5794cc7c94447e49f8833e8b4ec5e04f`, run `36254948563`: `test (ubuntu-latest)`, `test (macos-latest)`, and `test (windows-latest)` all `SUCCESS` (completed 16:19–16:22 UTC). That success does not prove the MySQL migration cases executed; the suite skips them when Docker MySQL is absent. A docs commit after that SHA needs a new CI run before merge.

Code review of `Migrate`, `ensureColumns`, and `backfillVerifiedIdentity`: additive columns, nullable verified identity, backfill only when `readme_snapshot_id` equals the accepted `snapshot_id`, the verified columns are still NULL, the JSON snapshot id matches, the all-period total is non-nil, and the date parses. A newer accepted snapshot is not copied into the verified columns. Unknown totals stay NULL. There is no `schema_migrations` table. `CREATE TABLE IF NOT EXISTS` does not alter an existing table; `ensureColumns` adds the missing columns.

Local disposable cases, this session, MariaDB 10.11.14 on `127.0.0.1`, database `wt_probe_077` (created for the run and dropped afterward; the pre-existing local `wheretoken` database was not migrated):

```text
go test ./internal/hosted/ -count=1 -run TestWallPublicationMigrationCases
ok
```

Cases covered by that test: fresh; v0.7.6-shaped; accepted newer than applied; missing historical verified total; pending failed publication. This is not production MySQL 5.7 and not a production backup dry run.

## Publication

CODE VERIFIED, this session, no database:

```text
go test ./internal/publicprofile/ -count=1 -run 'TestDecidePublicationMidnightBypassesCooldown|TestDecideRefreshMidnightBypassesCooldown|TestNextRefreshWakePrefersMidnight|TestApplyRefreshMidnightRolloverUpdatesAsOfDate|TestApplyRefreshMorningCatchUpIsNotAMidnightPublish'
ok
```

Those tests use a deterministic clock. They are not a production midnight publish.

PRODUCTION VERIFIED: no. The hosted binary is still the pre-v0.7.7 health string. No controlled publish was sent. The profile repository was not written.

Current public wall, read only, before any v0.7.7 publish. Repository `rainhuang0220/rainhuang0220`:

| File | Blob SHA | Commit | When |
| --- | --- | --- | --- |
| `README.md` | `c8d1c90778aa600c223d597c2df057a7ac01983c` | `d65f965d9ad6a8a55f269ef0a0be7ffd3d0903e6` | 2026-09-24T02:38:25Z |
| `wheretoken/preview-light.svg` | `9d28c07f5ac9aa66038bd07b6e04c5362dcf17fd` | `d7f0c5833f2e5ec4b0cb5d6cb5fd9dff4d9fa046` | 2026-09-24T02:38:21Z |
| `wheretoken/preview-dark.svg` | `d13cd3eaaf557be0dead9c4568797438c90a1660` | `9cf5e31dd592474698a2cc20554501b9a1886490` | 2026-09-24T02:38:24Z |

README image query `v=b8c90696a49ce62cdbeb620e2d2d4df99eca7cdb3b6f2d03c38cdb5b463d563d-6720ba3e714ed5a521e3cfcad145b740610024eeaed52208c033cd2657b6d0cf`. Alt text says partial coverage, updated September 24, 2026. Both SVG files contain the date `2026-09-24` and the snapshot hash `b8c90696a49ce62cdbeb620e2d2d4df99eca7cdb3b6f2d03c38cdb5b463d563d`. That snapshot hash and the asset hash `6720ba3e714ed5a521e3cfcad145b740610024eeaed52208c033cd2657b6d0cf` match the live hosted profile read in the same window. The three git commits are not one commit. The SVG blobs do not contain the asset hash. `verified_snapshot_id`, `verified_token_total`, and `verified_time` were not present on the public JSON and were not read from the production database.

No after-state. Token-threshold refresh was not run.

## Remaining limitations

| Item | Result |
| --- | --- |
| PR #10 is a superset of PR #9; land #10 once | PASS (branch comparison) |
| CI on `5d74b2a` | PASS. A later docs commit is a new run. |
| Migration cases A–E on disposable MariaDB | PASS. Not production. |
| Midnight clock tests | CODE VERIFIED. Not a production midnight. |
| Merge of #10 | BLOCKED. No merge tool. Repository `push: false` on the `gh` token. No merge SHA. |
| Close #9 | NOT DONE. Close it only after #10 is merged. |
| Git tag `v0.7.7` and GitHub Release | BLOCKED. Not created. `v0.7.6` was not moved. |
| In-repo formula sha256 for v0.7.7 | BLOCKED. No tag tarball. |
| Production backup | BLOCKED. SSH `Permission denied (publickey)`. |
| Production migration | BLOCKED. Same SSH denial. Not run. |
| Hosted deploy of a `VITE_HOSTED=1` build | BLOCKED. Same SSH denial. Health is still the old pseudo-version. |
| Public health of the current host | PASS for anonymous checks listed in the baseline. Authenticated login NOT TESTED. |
| Real GitHub wall on the new binary | NOT TESTED. Profile files were read only. |
| Homebrew tap | BLOCKED. Formula still `version "0.7.5"`, blob `0b124e415080eb4e3d1f627d888cdb8389de4343`. Tap permissions `push: false`. `brew` is not installed. No v0.7.7 `checksums.txt`. |

Final status: BLOCKED.
