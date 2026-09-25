# Public profile control plane

The public page stays at `https://rainhuang0220.github.io/whereToken/profile/`.
That URL is a static shell. Ordinary token changes do not need a new Pages
deployment, and choosing a palette does not publish anything.

## Authority

| State | Authority | What a visitor sees |
| --- | --- | --- |
| Usage | Hosted public projection, replaced after a successful sync | The shell's committed `profile.json` until a newer valid projection arrives |
| Palette | Hosted presentation (`cobalt`, `magenta`, `newsprint`) | The shell's `presentation.json` until the projection arrives |
| GitHub profile README | A materialized copy of the two preview SVGs plus three cache-keyed URLs | A static image. It cannot run JavaScript |

`snapshot_id` remains the usage identity. A palette change does not alter it.
The preview asset revision is a separate hash of the light SVG, the dark SVG,
and the palette.

The committed `public-profile/` bundle is the fallback. Its current usage
identity stays `sha256:15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62`
until a real local scan produces a different public snapshot. This document
does not treat that file as a live database.

## Freshness

This is near-real-time, not real-time. A token event becomes public only after
the existing scan and the existing device sync:

1. `wheretoken sync` scans, uploads the daily batch, then uploads the sanitized public snapshot.
2. `wheretoken scan` does the same upload when this device is already paired. The scan still succeeds if the upload fails.
3. While `wheretoken serve` is running, a paired device repeats that full sync every 15 minutes. There is no separate always-on daemon.
4. `wheretoken profile refresh` is a separate, default-off loop. It does not upload the daily batch. `profile refresh on` records only an enabled flag in `profile-refresh.json` next to `community.json`. The bare command runs one decide-and-PUT even while the switch is off. `profile refresh watch` repeats it in the foreground every 15 minutes. When the switch is on, the default report may PUT once after its scan. The watermark is the hosted envelope (`as_of_date`, all-time total, `freshness.updated_at`), not the scan index.

An offline scan does not replace the hosted projection. A snapshot whose public
status is `unavailable` does not replace it either. The inner snapshot keeps
`live_sync: false` because it is still a point-in-time sanitized document. The
API envelope around it says `near_real_time`.

The page shows `updated X min ago` and one of:

- `hosted` — a newer valid envelope replaced the fallback
- `committed snapshot` — only the static shell is loaded
- `hosted unavailable · committed snapshot` — the request failed or the envelope was rejected

## Public API

`GET /api/v1/public-profile/{login}`

The body is a `wheretoken.public-profile-live` envelope, schema version 1:

- `freshness.mode = near_real_time`
- `freshness.source = hosted`
- `freshness.updated_at`
- `data_revision` (the snapshot id)
- `presentation_revision` and `asset_revision`
- `presentation`
- `snapshot` — the existing public-profile document
- `owner.github_login`

It does not include prompts, paths, device tokens, credentials, job state, or
repository names. `PUT /api/v1/sync/public-profile` accepts a snapshot only
from the paired device bearer. The server runs the existing validator and
rejects private strings before storing the re-marshaled document.

Preview bytes for the interactive page are also available at:

- `GET /api/v1/public-profile/{login}/preview-light.svg`
- `GET /api/v1/public-profile/{login}/preview-dark.svg`

The GitHub README does not use those URLs.

## README materialization

The README cannot execute JavaScript, so it remains a static SVG. Generated
`preview-light.svg` and `preview-dark.svg` are committed in the profile
repository at `wheretoken/`, and the three existing preview references are
rewritten to:

`https://raw.githubusercontent.com/<owner>/<repo>/<branch>/wheretoken/preview-*.svg?v=<snapshot hex>-<asset hex>`

GitHub Camo caches by the full URL. The content-derived query is what changes
the image. `Cache-Control` is not treated as invalidation.

The product repository is not written. `wheretoken profile publish` remains
the local fallback for an offline or unrecovered owner.

Usage rewrites start only after one confirmed palette publish has updated the
README. Until that confirmation, sync refreshes the hosted projection and
leaves the profile README on its existing images.

After that, same-calendar-day usage rewrites wait:

- at least 30 minutes since the last README materialization
- then either a 100,000-token movement in the public all-time total, or six hours

A later local `as_of_date` on the newly accepted snapshot bypasses that wait and materializes immediately. It does not bypass projection replacement, validation, or the sensitive-string check, and it does not run when `readme_materialized_at` is still unset.

The hosted JSON and the README are intentionally split. `profile refresh` may PUT after 10,000 raw tokens and one hour, including several times in a day. Those uploads refresh the interactive snapshot and leave the README bytes alone until the coalesce rules or a local-date change. README git writes stay coarse.

A palette confirmation ignores that wait. Publishing the palette that is
already on the README returns `ALREADY_PUBLISHED` and does not create a commit.
If the README write fails after the hosted palette is saved, the job stays
`partial_failure` or `conflict` and a retry reads the remote files again. It
does not reuse a stale blob SHA and it does not force-push.

## Owner publish

GitHub OAuth still identifies the person and still discards the GitHub access
token. It requests no `repo` scope. The static page receives a one-time code
in the URL fragment, exchanges it for an opaque `wtp_1.` session, and keeps
that session in `sessionStorage` for the tab. It is not a GitHub token and it
is not stored in `localStorage`.

Repository writes use a GitHub App installation token on the server. The
browser sends only `{ "palette": "cobalt|magenta|newsprint", "confirm": true }`
plus `Authorization` and `X-CSRF-Token`. A body that includes `path`, `repo`,
or `branch` is rejected. The server writes only:

- `rainhuang0220/rainhuang0220` (the configured profile repository)
- branch `main`
- `README.md`
- `wheretoken/preview-light.svg`
- `wheretoken/preview-dark.svg`

The signed-in login must match both the URL owner and the repository owner.
Expected blob SHAs come from a read immediately before the write. A mismatch
is a conflict, not an overwrite.

Requested GitHub App permissions:

- Metadata: read
- Contents: read and write

Actions, Administration, and the product repository are not required for this
path. The app is not installed by this change.

## Local fallback

`wheretoken profile publish` and My Token on `127.0.0.1` still perform the
v0.7.4 two-repository publish. The public page links to that computer as
`本机发布`. Directly under the palette, the public page shows
`登录并发布到 GitHub 主页` before a profile session and
`发布到 GitHub 主页` for the signed-in owner. Choosing a color only previews
it (`预览中`). Confirmation stays on that page: `将 Cobalt 应用到 github.com/<login>`,
then `正在发布…` and `已发布`.

Visitors who are not the owner keep preview-only theme switching. An owner
session sees the published palette and the apply button. Job state is not
part of the public GET.
