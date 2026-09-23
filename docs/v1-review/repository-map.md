# whereToken v1 repository map

Date: 2026-09-16  
Reviewed revision: `de3dfb4`  
Release baseline: `v0.7.3`

## Product surfaces

whereToken is one accounting core presented through several deliberately
different products:

| Surface | Runtime | Data authority | Network behavior |
| --- | --- | --- | --- |
| CLI report | User machine | Local scanner | Cursor and Trae may call their own product APIs; otherwise local |
| Local dashboard | `127.0.0.1` | Local scanner | Same scan behavior as the CLI |
| Hosted web app | `wheretoken.plainlist.space` | Previously synchronized daily aggregates | GitHub OAuth plus explicit device synchronization |
| Public profile | Static host | Manually built, sanitized snapshot | No runtime API; reads only the bundled `profile.json` |
| GitHub Pages demo | Static GitHub Pages | Fabricated committed fixtures | No local access |
| Community Rank | User-configured self-hosted service | Anonymous uploaded daily totals | Disabled unless a service URL is configured |

These surfaces share domain logic where practical, but they do not share the
same privacy or freshness contract. The local scanner remains authoritative
for what happened on a machine. The hosted database and public profile are
projections, not alternate scanners.

## Top-level structure

| Path | Ownership |
| --- | --- |
| `cmd/wheretoken/` | CLI executable entry point |
| `cmd/wheretoken-hosted/` | Hosted API executable entry point |
| `internal/` | Go domain, adapters, application services, and renderers |
| `web/` | Vue dashboard shared by local, demo, and hosted build modes |
| `internal/webembed/dist/` | Committed local-dashboard build embedded in release binaries |
| `internal/profilewebembed/static/` | Canonical hand-maintained static public-profile page |
| `public-profile/` | Maintainer's explicitly published production snapshot |
| `docs/media/public-profile-demo/` | Synthetic public-profile demo bundle |
| `profile-e2e/` | Cross-browser public-profile tests |
| `site/` | Hand-written GitHub Pages landing page |
| `scripts/` | Build, fixture, profile, material, installer, and verification tools |
| `testdata/adapters/` | Desensitized source fixtures |
| `ci/github-workflows/` | Canonical workflow copies |
| `.github/workflows/` | Installed workflow copies consumed by GitHub |
| `npm/`, `Formula/`, `completions/` | Distribution wrappers and shell integration |

## Core Go ownership

### Accounting path

| Package | Responsibility |
| --- | --- |
| `internal/event` | Normalized `UsageEvent`, `TurnEvent`, quality, and derivation types |
| `internal/adapter` | Adapter contract, shared parsing helpers, and tool catalog |
| `internal/adapter/<tool>` | Source-specific discovery and parsing |
| `internal/scan` | Adapter orchestration, source status, windows, and dashboard summary JSON |
| `internal/index` | Disposable SQLite parse cache; never accounting authority |
| `internal/metric` | Request merge, token totals, calendar, drill-downs, and model aggregation |
| `internal/price` | Public list-price table, source catalog, and model resolution |
| `internal/vendor` | Provider normalization and labels |
| `internal/report` | CLI snapshot and terminal/JSON presentation |
| `internal/profile` | Deterministic local usage portrait |
| `internal/insight` | Deterministic explanatory observations |

The normalized total is owned by `internal/metric`:

```text
Total = Miss + Cache Read + Cache Create + Output
```

`Reasoning` is explanatory metadata and is not a second output charge.
`metric.CanonicalEvents` merges repeated rows by `Source + RequestID` using the
maximum of each token component. The complete request is assigned to its
latest timestamp for window and calendar consistency.

### Publication and compatibility

| Package | Responsibility |
| --- | --- |
| `internal/publicprofile` | Allowlisted schema-v2 snapshot, production validation, previews, and bundle manifest |
| `internal/profilewebembed` | Embedded HTML/CSS/JavaScript and Newsprint material for the static profile |
| `internal/card` | Legacy Vibe Coding Wall projection from the public-profile snapshot |

`internal/profile` and `internal/publicprofile` are intentionally different.
The first evaluates a portrait from metrics. The second defines what data may
be published and how a static bundle proves provenance, coverage, and asset
freshness.

### Hosted path

| Package | Responsibility |
| --- | --- |
| `internal/syncagg` | Privacy-safe daily-by-model payloads and source-scope hashing |
| `internal/hosted` | OAuth, web sessions, device pairing, MySQL storage, sync, and hosted summaries |
| `internal/credstore` | Local device credential storage |
| `internal/community` | Optional rank service and client, separate from hosted accounts |
| `internal/httpapi` | Localhost-only dashboard API |

The hosted executable does not register or run adapters. Scanning remains a
local CLI concern; hosted storage accepts aggregates only.

## Adapter registration

The tool list is intentionally small and explicit:

1. implementation under `internal/adapter/<id>/`;
2. metadata in `adapter.Catalog`;
3. runtime registration in `scan.Adapters`.

The registered tools are Claude Code, Kimi Code, Grok, MiniMax Agent,
OpenClaw, OpenCode, Codex, Cursor, Trae, Gemini CLI, Qwen Code, Cline, Roo
Code, Kilo Code, and ZCode. Cursor and Trae are the only cloud-enriched
adapters and honor offline mode.

The adapter contract is documented in `docs/adding-an-adapter.md` and
`docs/data-sources.md`. Discovery is not evidence of measurable usage.
Credential stores, mixed auth/transcript databases, prompts, and paths are
outside the normalized event contract.

## Data flows

### Local report and dashboard

```text
Agent JSONL / SQLite / product usage API
  -> Adapter.Discover
  -> Adapter.Parse
  -> UsageEvent + TurnEvent
  -> optional disposable index replay
  -> metric.Aggregate
  -> price / calendar / drill / portrait
  -> CLI report, JSON, or localhost dashboard
```

`internal/httpapi` serves the embedded Vue application and exposes
`GET /api/summary` plus `POST /api/scan`. Host, Origin, and Referer checks
protect the loopback boundary. A browser reload reuses the last result;
refresh explicitly starts a scan.

### Hosted synchronization

```text
Local scan
  -> canonical events
  -> daily x tool x vendor x model aggregates
  -> device-local or account-global scope
  -> HMAC source key
  -> authenticated sync batch
  -> MySQL rows
  -> hosted re-aggregation
  -> shared dashboard UI
```

Prompts, transcripts, paths, request/session identifiers, credentials, raw
events, and local database blobs are forbidden from the sync payload.
Cursor provider-API rows are account-global and replace across devices;
device-local ledgers sum across devices.

### Public profile

```text
One local scan
  -> canonical aggregation for all / today / 7d / 30d / 53w
  -> allowlisted schema-v2 snapshot
  -> production coverage and privacy validation
  -> profile.json + light/dark previews + static page + manifest
  -> explicit user commit and static deployment
```

The snapshot ID identifies public data and excludes generation time. The
asset revision identifies the preview and live-page renderer. A style-only
change therefore invalidates caches without pretending the underlying usage
changed.

### Demo generation

`scripts/gendemo` and `scripts/genprofiledemo` use fixed synthetic events and
the real aggregation path. CI never scans a runner's HOME. Demo and
maintainer bundles carry different machine-checkable provenance and deploy to
different URLs.

## Dashboard and theme systems

The main dashboard in `web/` is one Vue application with three build-time
modes:

| Mode | Flag | Data source |
| --- | --- | --- |
| Local | none | `/api/summary`, `/api/scan` |
| GitHub Pages demo | `VITE_DEMO=1` | committed `sample/*.json` |
| Hosted | `VITE_HOSTED=1` | `/api/v1/dashboard/summary` |

The dashboard identity is the kiln: the 53-week wall, the 窑 mascot, firing
language, a 2x5 readout, and restrained tabular detail. `web/src/themes` owns
eight full-dashboard palettes and their structural tokens. The theme gallery
previews a whole interface before applying a theme.

The public profile is a separate static application with its own compact
editorial design. Its Cobalt, Magenta, and Newsprint controls change activity
presentation rather than the main dashboard theme packs.

## Newsprint ownership

Production Newsprint currently uses a processed CC0 photoscanned material:

- upstream provenance and checksums:
  `scripts/gennewsprint/vendor/SOURCE.md`;
- local working color and displacement maps:
  `scripts/gennewsprint/vendor/`;
- deterministic processing:
  `scripts/gennewsprint/process.go`;
- canonical runtime asset:
  `internal/profilewebembed/static/assets/newsprint-surface.jpg`;
- live-page application:
  `internal/profilewebembed/static/assets/profile.css`;
- preview ink treatment:
  `internal/publicprofile/preview.go`.

The procedural height-field and crease-path iterations are retired. The
current runtime has one restrained paper asset, no fold SVG, no live shader,
and no turbulence overlay. Generated public bundles receive the canonical
asset through `publicprofile.Bundle`.

## Generated and committed artifacts

| Artifact | Source or generator | Publication rule |
| --- | --- | --- |
| `internal/webembed/dist/` | `web` local production build | Embedded in release binaries |
| `web/public/sample/*.json` | `go run ./scripts/gendemo` | Fabricated data only |
| `docs/media/public-profile-demo/` | `go run ./scripts/genprofiledemo` | Served at `/profile-demo/` |
| `public-profile/` | `wheretoken profile build` plus explicit asset refresh | Served at `/profile/` only with production provenance |
| Newsprint JPG | `go run ./scripts/gennewsprint` | Copied into profile bundles |
| `.github/workflows/` | `scripts/install-github-workflows.sh` from `ci/github-workflows/` | GitHub executes installed copies |
| Release archives/packages | `.goreleaser.yaml` on a version tag | GitHub Release |

There are no `go:generate` directives. Generators are explicit commands, and
tests catch several forms of source/generated drift. The repository still
requires maintainers to know which committed copy is canonical.

## Tests and CI

The repository has broad tests at the contract and regression boundaries:

- global and source-specific adapter tests;
- malformed-input and secret-leak fixtures;
- index append/truncate/replay invariants;
- adversarial request merge and overflow tests;
- report width, CLI behavior, and installer lifecycle tests;
- pricing provenance and unknown-rate tests;
- public-profile schema, privacy, production, and rendering tests;
- Playwright tests in Chromium, Firefox, and WebKit;
- Vue unit tests for state, interaction, and all three build modes;
- hosted auth, pairing, sync, storage, and account tests.

GitHub CI runs on Linux, macOS, and Windows. It checks formatting, vet, Go
tests, selected race suites, cross-platform builds, web tests, browser tests
on Linux, npm-wrapper tests, CLI fixtures, Windows same-shell installation,
and `govulncheck`.

The local documented quality gate is:

```bash
go test ./...
go vet ./...
cd web && npm ci && npm test
cd profile-e2e && npm ci && npm test
bash scripts/verify-cli.sh
```

## Deployment and release

- GitHub Pages assembles `site/`, the demo SPA, the synthetic profile demo,
  and an optional provenance-gated maintainer profile.
- The hosted binary and `VITE_HOSTED=1` SPA are built by
  `scripts/build-hosted.sh`; nginx serves the SPA and reverse-proxies `/api/`
  to a loopback Go process.
- `wheretoken serve` remains loopback-only and is not the hosted server.
- A git tag is release truth. GoReleaser creates six platform archives plus
  package-manager artifacts.
- The in-repository formula, external Homebrew tap, npm wrapper, and
  maintainer profile have separate post-release update steps.
- HEAD contains public-profile and Newsprint work newer than `v0.7.3`; those
  changes are not in the latest release binary until a later patch is cut.

## Recent decision history

The recent history shows a compressed sequence of substantial product work:

1. pricing, portrait, and hosted sync were added around `v0.6`-`v0.7`;
2. public profile schema and publication gates landed in `v0.7.1`-`v0.7.3`;
3. the public profile then received a major editorial redesign;
4. selectable profile palettes followed;
5. Newsprint was rebuilt repeatedly, moving from fold paths, to procedural
   material, to a processed photoscanned source;
6. Windows installer semantics were fixed and locked with same-shell tests.

This history explains both the strong regression coverage and the current
need to reduce product narrative churn before declaring v1.

## Source-of-truth index

| Concern | Source |
| --- | --- |
| Token arithmetic | `docs/token-accounting.md`, `internal/metric` |
| Adapter behavior | `docs/data-sources.md`, adapter package tests |
| Supported-tool registry | `adapter.Catalog`, `scan.Adapters` |
| Price rates | `internal/price/table.go` |
| Price provenance | `internal/price/catalog.go` |
| Public snapshot contract | `docs/public-profile.schema.json`, `internal/publicprofile` |
| Public-profile renderer | `internal/profilewebembed/static`, `internal/publicprofile/preview.go` |
| User portrait | `internal/profile` |
| Community Rank | `docs/community.md`, `internal/community` |
| Hosted design and code | `internal/hosted`, `internal/syncagg`; ADR is historical |
| Release version | Git tag |
| CI workflow source | `ci/github-workflows/` |

## Boundary and maturity risks

1. **Public privacy language is inconsistent.** The README, landing page, and
   `SECURITY.md` emphasize that analytics do not leave the machine, while the
   same project now promotes a hosted app whose explicit sync uploads daily
   aggregates. Both claims can be true only when the modes are named and
   separated clearly.
2. **The hosted ADR is stale.** It is marked proposed and describes
   pre-implementation facts even though the hosted implementation shipped in
   `v0.7.0`.
3. **The product has two theme systems.** Dashboard glaze themes and
   public-profile activity palettes are valid separate concepts, but their
   names and documentation can make them appear like one system.
4. **Generated ownership is learned rather than obvious.** The committed web
   build, three profile copies, dual workflow trees, and generated previews
   are protected by tests but still add maintainer cognitive load.
5. **There are two related summary JSON contracts.** CLI report JSON and
   dashboard scan JSON share domain calculations but intentionally expose
   different presentation fields.
6. **Profile seeds differ by context.** Local views use an install identity;
   hosted views use an account seed. The trait engine is shared, but wording
   is not guaranteed to match across those surfaces.
7. **The index is deliberately imperfect.** Same-size in-place rewrites with
   restored metadata can replay stale cached events until `rebuild`.
8. **Adapter completeness is source-specific.** Global contract tests guard
   safety and shape, while zero-versus-unavailable semantics still require
   focused tests per provider.
9. **Release and main differ.** Visual improvements on main are not evidence
   of the currently downloadable binary's behavior.
10. **The project still labels itself Alpha.** A v1 declaration requires a
    deliberate decision about hosted reliability, signing, primary language,
    and which surface represents the product.

## Architecture that should be treated as load-bearing

The review phase must assume the following are intentional unless concrete
evidence proves otherwise:

- local ledgers remain accounting authority;
- normalized events are the adapter boundary;
- missing usage remains unavailable, never zero;
- reasoned request merging and canonical calendar assignment remain shared;
- pricing remains optional, model-level, and sourced only from public cards;
- local, hosted, demo, public-profile, and community network contracts remain
  explicit;
- the public snapshot stays allowlisted and manually published;
- the local dashboard remains loopback-only;
- the index remains a disposable accelerator;
- the kiln wall and honest-accounting language are the strongest existing
  product identity.
