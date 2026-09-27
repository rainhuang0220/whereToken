# Merge checklist for PR #10

Observed 2026-09-26, after `git fetch origin main cursor/release-v077-3e3c cursor/unified-github-wall-3e3c`. This session did not merge, tag, or deploy.

Landing pull request: [#10](https://github.com/rainhuang0220/whereToken/pull/10) `cursor/release-v077-3e3c`, title `Prepare v0.7.7 release`. It is open and still a draft.

## CI on the head that was verified

Head at verification: `a9b2e6c205dbe353f61fdcd568a0eedc22e64520`.

Run [36260309336](https://github.com/rainhuang0220/whereToken/actions/runs/36260309336) (`ci`, event `pull_request`) completed with every job `SUCCESS`. `gh pr checks 10` reported `pass` for each check.

| Check | Conclusion | Completed (UTC) | Job |
| --- | --- | --- | --- |
| `test (ubuntu-latest)` | pass | 2026-09-26T17:52:17Z | [108454734715](https://github.com/rainhuang0220/whereToken/actions/runs/36260309336/job/108454734715) |
| `test (macos-latest)` | pass | 2026-09-26T17:50:13Z | [108454734717](https://github.com/rainhuang0220/whereToken/actions/runs/36260309336/job/108454734717) |
| `test (windows-latest)` | pass | 2026-09-26T17:51:29Z | [108454734558](https://github.com/rainhuang0220/whereToken/actions/runs/36260309336/job/108454734558) |

The run log also has non-failing annotations (Node.js 20 deprecation, and cache-restore `tar` exit 2 on ubuntu and macos). Those annotations did not change the job conclusions.

This file is a later docs commit on the same branch. Its own Actions run does not exist until GitHub starts it. Before merging, run `gh pr checks 10` on the head GitHub shows and wait until those three `test` checks pass on that head.

## Required checks

`GET /repos/rainhuang0220/whereToken/branches/main/protection` returned HTTP 403 `Resource not accessible by integration`. `GET /repos/rainhuang0220/whereToken/rules/branches/main` returned `[]`. The token's `X-Accepted-Github-Permissions` is `metadata=read`, so that empty list does not prove the repository has no required status checks. The checks that ran on this head are the three `test` jobs from `.github/workflows/ci.yml`.

## Mergeability

`gh pr view 10`: `mergeable` `MERGEABLE`, `mergeStateStatus` `CLEAN`, `state` `OPEN`, `isDraft` `true`.

Base is `main` at `b4c9f81252656c0524208db019771344dfda2658`, which matches `origin/main` after the fetch. No merge conflicts were reported.

## Who merges, and how

The owner merges in the GitHub UI, pull request #10 only.

The PR is a draft. In that UI, mark it ready for review, then use the merge control for #10. Do not merge pull request #9. #9 (`cursor/unified-github-wall-3e3c`, head `695bde29c77422d68db744e8992783ce97f5e121`) is an open draft and an ancestor of `a9b2e6c205dbe353f61fdcd568a0eedc22e64520`. Leave #9 open. Do not close it from this checklist.

This session cannot merge. `gh api repos/rainhuang0220/whereToken` returned `admin`, `maintain`, `pull`, `push`, and `triage` all false. `X-Accepted-Github-Permissions` is `metadata=read`. `GET /user` returned HTTP 403 `Resource not accessible by integration`. `gh` is read-only here. Do not push `main`, do not force-push, and do not call the merges API.

## Result of the merge

The merge commit SHA is unknown until GitHub creates it. Do not invent one.

The merge contains the PR head that is current at merge time. At this verification, that line of history includes `a9b2e6c205dbe353f61fdcd568a0eedc22e64520`. The current base is `b4c9f81252656c0524208db019771344dfda2658`.

After the UI merge:

```bash
git fetch origin main
git merge-base --is-ancestor a9b2e6c205dbe353f61fdcd568a0eedc22e64520 origin/main
git rev-parse origin/main
```

`origin/main` is then the merge commit GitHub created. Record that SHA. It is not known in advance.

## Tag v0.7.7 without moving v0.7.6

`v0.7.6` is an annotated tag. The tag object is `3f24faa83c9f601f83aa9296cee76382023f78fa`. It peels to `42f22dcdf85502937e063a8595846d78e35d691d`. `git ls-remote --tags origin` listed `v0.7.6` and did not list `v0.7.7`. `gh release view v0.7.7` returned `release not found`. The latest GitHub Release is still `v0.7.6` (published `2026-09-26T05:14:28Z`).

Before and after tagging, both of these must stay unchanged:

```bash
git rev-parse 'v0.7.6^{commit}'
# 42f22dcdf85502937e063a8595846d78e35d691d
git rev-parse v0.7.6
# 3f24faa83c9f601f83aa9296cee76382023f78fa
```

Do not run `git tag -f` on `v0.7.6`. Do not delete that tag.

Annotated tag on the merge commit only, after `origin/main` is that commit:

```bash
git fetch origin main
git rev-parse origin/main
git tag -a v0.7.7 "$(git rev-parse origin/main)" -m "v0.7.7"
git push origin v0.7.7
git rev-parse 'v0.7.6^{commit}'
```

`docs/releasing.md` shows a lightweight `git tag vX.Y.Z`. For this release the owner tag is the annotated command above. Either tag name matches the workflow trigger. Do not push the tag from this session.

## How the GitHub Release is published

Pushing the tag triggers `.github/workflows/release.yml` (`on.push.tags: ["v*"]`). The `goreleaser` job runs `goreleaser release --clean` and publishes the GitHub Release (six archives, `checksums.txt`, `.deb`, `.rpm`). `.goreleaser.yaml` stamps `-X main.version={{.Version}}` on `./cmd/wheretoken` only. It does not build `cmd/wheretoken-hosted`.

Do not create the Release with `gh release create`. The workflow is the publisher.

The hosted site is a separate step in `engineering/v0.7.7_operator_runbook.md`. The tag push does not deploy `wheretoken.plainlist.space`.

## Later, after the Release exists

`docs/releasing.md` then updates `Formula/wheretoken.rb` from the tag tarball sha256, and the `rainhuang0220/homebrew-wheretoken` tap from that Release's `checksums.txt`. The tap formula is still `version "0.7.5"` (blob `0b124e415080eb4e3d1f627d888cdb8389de4343`). Do not invent a checksum. Do not update the tap in the merge commit. Until the tap is bumped, brew users stay on the old formula.
