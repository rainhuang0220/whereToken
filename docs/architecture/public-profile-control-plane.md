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
3. While `wheretoken serve` is running, a paired device repeats that full sync every 15 minutes.
4. `wheretoken profile refresh` is a separate sanitized PUT. It does not upload the daily batch. The default report and the default install do not run it. `profile refresh on` flushes the switch file with the agent enabled, and only then, on macOS, installs a user launchd agent that runs `profile refresh watch --quiet`. A failed bootstrap leaves the switch on. `off` disables the switch and uninstalls the agent. Linux and Windows do not install a background agent. The bare command is one explicit PUT even while the switch is off; launchd does not run that bare command. `watch` is the only loop. It re-reads the switch on every wake, including the first, and exits when the switch is off. Wakes are the sooner of 15 minutes and the next owner-local midnight. A powered-off Mac does not publish. Catch-up is the next wake of that long-running watch, or RunAtLoad after a graphical login. SSH with no GUI session does not run the agent; `profile refresh status` can show `scheduler=dead`.

The GitHub wall watermark is the last publication whose README and both preview SVGs were read back and matched. That verified snapshot id, raw total, owner-local date, and publication instant are not the same thing as the latest accepted hosted projection. A missing verified total stays unknown and is not treated as zero. Only a strictly later owner-local calendar day is a daily publication, and that day does not wait for the one-hour usage cooldown. Usage growth publishes after 10,000 raw tokens and one absolute hour since that verified publication. A smaller total is not published. The watch still checks about every 15 minutes and also wakes at the next owner-local midnight.

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

The first README write does not wait for a theme publish. It uses the
verified palette when one exists, and newsprint otherwise. Accepting a
snapshot stores the projection even if the GitHub write fails. A failed
README does not roll back that projection, and a rejected projection does
not change a README that already applied. The hosted process retries a
failed README without a new client PUT. `desired_snapshot_id` is stored
when the projection is accepted. The applied snapshot id, cache key, and
`applied_at` are stored only after GitHub read-back succeeds.

Automatic GitHub publication uses one rule, `DecidePublication`, on every entry point:

- a strictly later owner-local date publishes immediately, including zero token growth
- otherwise the raw all-time total must be at least 10,000 above the verified total and one absolute hour must have passed since that verified publication
- an explicit palette publish still writes immediately and, once verified, becomes the new cooldown anchor
- a crossing that happens during the hour is stored once and published by the hosted maintenance loop without another client PUT

An earlier date, an empty date, and an invalid date do not publish. A missing verified total is not zero. A failed apply keeps the newest desired snapshot and retries that one snapshot, not every intermediate PUT. A remote conflict leaves the projection in place, does not mark the README verified, and does not report the theme as published. Theme publish still promotes the palette only after GitHub verifies the files.

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
