# Public deployment

whereToken has two surfaces that must never be conflated:

| Surface | Where it runs | What it can see |
| --- | --- | --- |
| Local CLI + dashboard (`wheretoken`, `wheretoken serve`) | The user's machine, `127.0.0.1` only | Real local ledgers, paths, workspaces — everything stays local |
| Public site (<https://rainhuang0220.github.io/whereToken/>) | GitHub Pages, static only | Nothing. Landing page + fabricated demo data |

The local scanner is never exposed as a public server, and there is deliberately
**no public Community Rank cluster** (see `docs/community.md` — a public URL is a
documented non-goal of the v1 threat model).

## What the public site is

- `site/` — a hand-written static landing page, no build step.
- `web/` in **demo mode** — the real dashboard compiled with `VITE_DEMO=1`:
  - `fetchSummary`/`rescan` read committed sample payloads from
    `web/public/sample/{all,today,7d,30d}.json` instead of `/api/*`;
  - community actions are inert; the refresh control is hidden;
  - the status line reads 演示数据.
- Everything is served from `https://rainhuang0220.github.io/whereToken/`
  (`/demo/` for the dashboard). There is no public API, no upload endpoint,
  no server-side state.

## Demo sample data

`web/public/sample/*.json` is fabricated by a generator — no real user data
ever touches it:

```bash
go run ./scripts/gendemo        # rewrites web/public/sample/
```

It seeds a fixed PRNG, invents events across eight tools over ~45 days, runs
them through the real `metric.AggregateAt` + `scan.ApplyWindow` pipeline, and
marshals with the same `scan.MarshalSummary` the local server uses, so the
payload shape cannot drift silently from what the dashboard renders. The
samples are committed; regenerate them when the summary schema changes.
`web/src/sample.test.ts` loads all four payloads through the real grid
selectors so a stale or broken sample fails CI.

## CI/CD

`ci/github-workflows/pages.yml` (installed to `.github/workflows/` by
`scripts/install-github-workflows.sh`) runs on pushes to `main` that touch
`site/`, `web/`, or the generator:

1. `npm ci && npm test` — the demo build is gated on the full web test suite.
2. `VITE_DEMO=1 npm run build` — demo dashboard with base `/whereToken/demo/`.
3. Assemble `_pages/` = `site/` + `web/dist` under `/demo/` + a `404.html`
   copy of the demo shell for SPA deep links.
4. `actions/deploy-pages` publishes to the `github-pages` environment.

There is no separate preview channel: pull requests are covered by `ci.yml`
(which runs the same web tests), and Pages deploys only from `main`. No
secrets are involved — Pages uses the automatic `GITHUB_TOKEN` plus the
`pages`/`id-token` permissions in the workflow.

## Privacy posture of the deploy

- The artifact is pure static HTML/JS/JSON; GitHub Pages adds no server logic.
- Sample payloads contain only invented `demo/*` workspace names and UUIDs-free
  identifiers (`demo-<source>-<n>`); no absolute paths, prompts, credentials,
  or real usage ever enter them (grep-checked before every release of the
  site, and the generator has no code path that reads the host).
- The demo build cannot upload anything: the community POST path is a no-op
  under `VITE_DEMO=1` and the sample's community view is the honest
  `service_unconfigured` state.
