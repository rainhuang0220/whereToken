# Public Profile

`wheretoken profile build <dir>` writes a **public snapshot** of local coding-agent usage: JSON, GitHub-light and GitHub-dark preview SVGs, and a static live page. The interactive page is live in the browser. The usage data itself is a locally generated public snapshot, not a live cloud sync.

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

`all` means all history available to this scan, not a promise of identical all-time history across providers. The page labels it “Available history” and shows source-specific account windows next to account-derived rows. The hero total is the sum of tracked values; `data_status=partial` and the coverage panel disclose any incomplete source.

## Preview freshness

Every snapshot contains a deterministic `sha256:…` `snapshot_id`. The hash excludes only `generated_at`, so rebuilding unchanged data at a later minute keeps the same ID; changing public data changes it. `manifest.json`, `profile.json`, and both previews carry the same ID.

Use the hex part as a cache-busting query in Profile READMEs:

```html
<source media="(prefers-color-scheme: dark)" srcset="https://example/profile/preview-dark.svg?v=SNAPSHOT_HEX">
<img src="https://example/profile/preview-light.svg?v=SNAPSHOT_HEX" alt="whereToken public usage preview">
```

An explicit publication updates the query only when the snapshot ID changes. This makes GitHub Camo fetch a new preview without pretending the snapshot is live-synced.

## Compatibility

`wheretoken card <path.svg>` still writes the 800×576 Vibe Coding Wall. It now projects from the same `ProfileSnapshot`. Prefer `profile build` for new READMEs.
