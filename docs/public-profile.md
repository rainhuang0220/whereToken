# Public Profile

`wheretoken profile build <dir>` writes a **public snapshot** of local coding-agent usage: JSON, GitHub-light and GitHub-dark preview SVGs, and a static shell. The shell renders immediately from that committed snapshot. On the maintainer's GitHub Pages profile it then requests the hosted projection and replaces the snapshot only when a newer valid envelope arrives. That refresh is near-real-time after `wheretoken sync` or a paired `wheretoken scan`, not a push for every token event. See [`docs/architecture/public-profile-control-plane.md`](architecture/public-profile-control-plane.md).

A real personal profile must be built **online** so cloud-enriched sources (Cursor, Trae) can read their account usage APIs:

```bash
wheretoken profile build ./public-profile
wheretoken profile validate ./public-profile --production
```

`--offline` is for CI fixtures, tests, and explicitly local-only snapshots. It skips Cursor/Trae account APIs. If local Cursor rows still have requests, token columns stay **unavailable** (`—`) or explicitly **partial** when degraded local token counts exist, never a fabricated authoritative `0`. Publishing that as production fails `--production` unless you pass `--allow-partial`.

Cursor filtered usage is accepted only when every advertised page completes. A mid-pagination error or page-limit truncation discards the partial account rows; a complete aggregated response may recover the total. Otherwise coverage records an enumerated failure and production validation fails. Raw API errors and response bodies never enter the snapshot.

Copy the directory to a static host. The CLI never logs into GitHub, never commits, and never pushes.

This repository keeps three artifacts separate:

- `docs/media/public-profile-demo/` is a committed synthetic product demo and is served only at `/whereToken/profile-demo/`. Its JSON, preview, and page say `DEMO DATA` / `synthetic_demo`.
- Any directory produced by `profile build` is a local real bundle. It remains private until its owner explicitly publishes it.
- `public-profile/` is the maintainer's production source. Pages copies it to `/whereToken/profile/` only when `profile.json` and `manifest.json` both declare `local_sanitized_snapshot`; it never falls back to demo data.

The production flow is therefore private ledger → local release binary → sanitized bundle → explicit Git commit. GitHub Actions never scans HOME.

Default build omits model breakdown and cost. Opt in:

```bash
wheretoken profile build ./public-profile --include-models --include-cost
```

`--today` / `--since` are rejected: the bundle always contains `all`, `today`, `7d`, `30d`, and `53w` from one scan and one clock.

Schema: [`docs/public-profile.schema.json`](./public-profile.schema.json) (version **2**). Version 1 snapshots still validate in normal compatibility mode, but fail `--production` because they cannot prove the v2 per-source coverage contract; rebuild them with the current binary before publishing. Token math is unchanged (`docs/token-accounting.md`). Missing usage is unavailable (`—`), never `$0`. Each breakdown row carries `coverage` (`tokens` / `requests` / `token_source` / `token_window` / `reason`). Activity series are explicit `{dimension, id, metric: tokens|requests}` — the live page never guesses, and never falls back from a missing agent series to All.

`activity.series[].levels` remains a five-level compatibility field in the public schema. It is not the renderer's visual granularity: preview rendering derives a continuous intensity from each raw `values[]` entry with the stable absolute scale `sqrt(clamp(value / 1,000,000,000, 0, 1))`. Zero stays empty; only days at or above one billion tokens saturate, and the same daily value renders at the same intensity regardless of the surrounding 53-week distribution.

`all` means all history available to this scan, not a promise of identical all-time history across providers. The page labels it “Available history” and shows source-specific account windows next to account-derived rows. The hero total is the sum of tracked values; `data_status=partial` and the coverage panel disclose any incomplete source.

## Preview freshness

Every snapshot contains a deterministic `sha256:…` `snapshot_id`. The hash excludes only `generated_at`, so rebuilding unchanged data at a later minute keeps the same ID; changing public data changes it. `manifest.json`, `profile.json`, and both previews carry the same ID.

The bundle manifest also contains a content-derived `asset_revision`. It hashes both preview SVGs, `presentation.json`, and the embedded HTML/CSS/JavaScript, so a renderer-only, palette-only, or style-only publication changes the asset revision without changing the data identity.

`presentation.json` is the published palette (`cobalt`, `magenta`, or `newsprint`). It is not usage data. `wheretoken profile palette <id>` saves the owner default on this machine; `wheretoken profile build <dir>` reads that file unless `--public-palette` is set. A visitor's browser and `?palette=` can override the view. They do not rewrite the file. My Token → 公开 Profile can save the same file locally. `wheretoken profile publish` shows the two-repository plan and does not push; `profile publish --yes` is the explicit approval that fast-forwards the product checkout and then the personal README. See [`docs/architecture/public-profile-theme-publication.md`](architecture/public-profile-theme-publication.md).

Use `SNAPSHOT_HEX-ASSET_HEX` as the cache-busting query in Profile READMEs:

```html
<source media="(prefers-color-scheme: dark)" srcset="https://example/profile/preview-dark.svg?v=SNAPSHOT_HEX-ASSET_HEX">
<img src="https://example/profile/preview-light.svg?v=SNAPSHOT_HEX-ASSET_HEX" alt="whereToken public usage preview">
```

An explicit publication updates the query when either the public data or the renderer assets change. This makes GitHub Camo fetch a new preview while preserving `snapshot_id` as a data-only identifier.

GitHub renders a README SVG through Camo as one image element. The SVG can keep a root title and description, but per-cell pointer or keyboard interaction is not available inside the README image. Link the static preview to the interactive page instead of attempting image maps, scripts, or hundreds of HTML cells.

## Compatibility

`wheretoken card <path.svg>` still writes the 800×576 Vibe Coding Wall. It now projects from the same `ProfileSnapshot`. Prefer `profile build` for new READMEs.
