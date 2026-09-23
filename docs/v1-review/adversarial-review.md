# Adversarial v1 Review — Reviewer C

**Repository:** `/workspace`  
**Revision:** `de3dfb4`  
**Branch:** `cursor/v1-maturity-review-27f1`  
**Baseline release:** `v0.7.3`

## Verdict

whereToken is not ready to call itself v1.

The repository is trying to ship a local accounting tool, local dashboard, hosted account product, synchronization protocol, public-profile generator, GitHub Pages demo, anonymous leaderboard, personality-like portrait engine, two theme systems, and a custom photoscanned paper-material pipeline at once.

Those surfaces do not reinforce one product. They multiply privacy contracts, storage authorities, release paths, generated artifacts, test requirements, terminology, and failure modes. Several hosted failures can silently preserve incorrect token data or strand device credentials. Those are release blockers, not aesthetic objections.

The smallest defensible v1 is the local CLI, JSON output, and loopback dashboard. Hosted sync, Community Rank, public profiles, portraits, and Newsprint should not be in the v1 scope.

## Release blockers

### 1. Equal-revision synchronization can silently ignore changed usage

`internal/hosted/store.go` implements equal-revision conflict detection in `Store.ApplyUsage`. It queries only:

```sql
SELECT revision, miss FROM usage_daily_model
```

When revisions match, it compares only `row.Miss`:

```go
if row.Revision == storedRev.Int64 {
    if row.Miss != storedMiss.Int64 {
        return ErrRevisionClash
    }
    continue
}
```

Changes to any of these fields are silently ignored when `miss` remains unchanged:

- `cache_read`
- `cache_create`
- `output`
- `requests`
- `user_turns`
- `quality`
- `derivation`

This directly contradicts the contract in `docs/architecture/hosted-web-v070.md`, which says equal revision plus different measures must return `409`.

The test in `internal/hosted/store_test.go`, `TestApplyUsageRejectsStaleRevision`, varies only `Miss`, reproducing the implementation’s blind spot instead of testing the full row.

This is an accounting-integrity defect. A hosted dashboard can diverge from the local authority while accepting the synchronization request.

### 2. Device pairing loses credentials on restart or a dropped response

The pairing challenge is persisted, but the raw device credential is not. In `internal/hosted/pair.go`:

1. `pairConfirm` creates a device in MySQL.
2. It marks the challenge consumed.
3. It puts the only recoverable raw token in an in-process `sync.Map`.
4. `pairStatus` removes the token with `LoadAndDelete` before writing the HTTP response.

Consequences:

- A process restart after confirmation loses the token permanently.
- A dropped response after `LoadAndDelete` makes a retry return `consumed`.
- The user must pair again.
- The database retains an orphan device because only its token hash was persisted.

There is also a confirmation race. `Store.ConsumePair` in `internal/hosted/store.go` ignores `RowsAffected`. Two confirmations can both call `InsertDevice`; the losing conditional update affects zero rows but still returns success. The later `putPending` can overwrite the earlier pending credential.

The tests in `internal/hosted/pair_test.go` prove only the intended single-process, single-request sequence. They do not cover restart recovery, response loss, concurrent confirmation, or zero-row consumption.

The rate-limit claims in `docs/architecture/hosted-web-v070.md` also do not match the implementation. The ADR promises status limiting per challenge and confirmation limiting per user. `internal/hosted/pair.go` limits status by IP, while `pairConfirm` has no corresponding limiter.

This flow is not reliable enough to issue credentials.

### 3. Batch idempotency excludes part of the request and is not atomic

`Store.SyncBatch` in `internal/hosted/store.go` hashes only `batch.DailyModelUsage`:

```go
body, err := json.Marshal(batch.DailyModelUsage)
sum := sha256.Sum256(body)
```

The hash excludes:

- `Sources`
- `Timezone`
- `GeneratedAt`
- `PriceCatalogVersion`
- other batch metadata

Reusing an idempotency key with identical usage rows but changed source status or timezone returns success early and silently ignores those changes. That is not “identical key + identical body,” despite the contract in `docs/architecture/hosted-web-v070.md`.

The operation is also split across transactions:

1. `ApplyUsage` commits usage rows.
2. Timezone is updated separately.
3. Source states are updated separately.
4. The idempotency record is inserted last.

Timezone and source-state errors are explicitly discarded:

```go
_, _ = s.db.ExecContext(...)
```

A request can therefore return success with only part of its state applied. A crash or concurrent retry can also occur after usage commits but before the idempotency record exists.

The test `TestIdempotentBatchRetry` in `internal/hosted/store_test.go` checks only that one simple usage row is not doubled. It does not test the actual request envelope, failure atomicity, concurrent reuse, source states, or timezone.

### 4. CI can report green while all MySQL-backed hosted tests are skipped

`internal/hosted/mysqltest.go` skips storage tests whenever Docker or MySQL setup fails unless `WHERETOKEN_REQUIRE_MYSQL` is set:

```go
if os.Getenv("WHERETOKEN_REQUIRE_MYSQL") != "" {
    t.Fatal(sharedStoreErr)
}
t.Skip(sharedStoreErr.Error())
```

`.github/workflows/ci.yml` does not:

- provision a required MySQL service;
- set `WHERETOKEN_MYSQL_TEST_DSN`;
- set `WHERETOKEN_REQUIRE_MYSQL`;
- assert that hosted storage tests actually ran.

The selected race suite also excludes `internal/hosted`.

The most consequential service in the repository therefore has optional integration tests. This blocks hosted sync from being treated as production functionality.

### 5. Public privacy documentation omits the hosted upload path

`README.md` and `SECURITY.md` explain local analytics and Community Rank network behavior. Neither describes the shipped hosted account synchronization path.

`README.md` says:

- local analytics stay on the machine;
- Community Rank is the conditional upload;
- the public profile is not live cloud sync.

It does not explain `wheretoken login`, `wheretoken sync`, GitHub OAuth, device credentials, or the daily-by-model aggregate uploaded to the hosted service.

`SECURITY.md` likewise documents local scanning and Community Rank but says nothing about:

- hosted synchronization;
- uploaded fields;
- account identity;
- retention and deletion;
- device revocation;
- hosted breach reporting;
- who operates `wheretoken.plainlist.space`.

“Local-first” and optional hosted synchronization can coexist. Omitting the hosted mode from the privacy explanation prevents informed consent.

### 6. The only hosted architecture document describes a different repository state

`docs/architecture/hosted-web-v070.md` is marked:

> Status: Proposed (audit complete, implementation next)

The implementation has already shipped. The document also records obsolete commit hashes, old command availability, machine paths, a live server IP, SSH username and key behavior, sibling services, ports, database versions, disk usage, and deployment topology.

An IP address or SSH key filename is not itself a credential, but publishing unnecessary operational inventory is poor security hygiene. More importantly, maintainers cannot tell which portions are current design, historical planning, or abandoned assumptions.

A v1 cannot depend on an ADR that announces “implementation next” after deployment.

### 7. Distributed binaries are unsigned

`README.md` and `docs/macos-signing.md` explicitly state that release binaries remain unsigned. `.goreleaser.yaml` makes macOS signing optional and has no Windows Authenticode path.

This project recommends piping a remote installer into a shell and distributes an updater. Checksums hosted beside unsigned archives do not provide an independent trust root if the release account or artifacts are compromised.

Calling these artifacts v1 while leaving signing conditional is not credible release engineering.

## Product-scope failures

### Hosted web does not earn its complexity

The hosted product adds:

- `cmd/wheretoken-hosted/`
- `internal/hosted/`
- `internal/syncagg/`
- `internal/credstore/`
- GitHub OAuth and sessions
- CSRF handling
- device pairing and revocation
- MySQL migrations
- HMAC-derived source identities
- account-global versus device-local merge rules
- another dashboard authority
- hosted-specific branches in the shared Vue application
- bespoke deployment through `scripts/build-hosted.sh`

That would require production-grade durability, observability, migrations, incident response, deletion semantics, privacy documentation, and mandatory database tests. The repository currently supplies none of those as a coherent operated-service contract.

The hosted feature also weakens the product’s clearest boundary: read ledgers locally and present the result locally. It turns a local utility into an account service without demonstrating a user need that cannot be met by local export.

Remove hosted sync from v1. If it remains experimental, mark it as such, hide it from stable distribution, and stop treating the stale ADR as documentation.

### Community Rank is a test harness presented as a feature

`docs/community.md` admits that:

- there is no public rank cluster;
- rankings are self-reported and unaudited;
- fake token totals are accepted;
- Sybil participation is accepted;
- the UUID is a bearer capability;
- “all-time” means only days the client happened to upload;
- the result is not shown in the normal report or dashboard.

`internal/cli/community.go` makes `wheretoken community serve` use:

```go
community.NewHandler(community.NewStore(...))
```

That store is in memory. Every restart discards the board.

The command binds to `127.0.0.1`, so operating a shared service also requires undocumented proxy or deployment work. A local, volatile leaderboard containing attacker-controlled totals is not a meaningful product surface.

Delete Community Rank from v1.

### The portrait is arbitrary copy attached to threshold buckets

`internal/profile/vector.go` derives a small set of thresholded traits. `internal/profile/phrases.go` then contains roughly 200 Chinese phrases, with an anonymous seed selecting synonyms such as:

- “模型游牧”
- “模型自助餐”
- “缓存红利吃满”
- “算力预算充足”
- “主力生产工具”

The seed does not encode behavior. Two users with the same measurements can receive different wording solely because their install identifiers differ. The variation is decorative randomness presented as personalization.

Local and hosted contexts use different seeds, so the same user and metrics are not guaranteed to receive the same wording across surfaces. Cost-derived phrases also turn an API-equivalent estimate into claims such as “budget sufficient,” which is not supported by the data.

This is not a portrait. It is a phrase lottery over a few summary statistics. Replace it with direct facts or remove it.

There is also a second overlapping mechanism. `internal/scan/scan.go` emits both `evaluation` and `portrait`. `web/src/kiln.test.ts` explicitly asserts that no evaluation cell exists, while `web/src/sample.test.ts` still requires evaluation data in every fixture. The API and tests maintain a feature the UI deliberately does not render.

Delete `evaluation` from the dashboard contract and delete the portrait from v1.

### Eight dashboard themes are mostly palette inventory

`web/src/themes/manifest.ts` defines eight dashboard themes:

- `kiln`
- `moss`
- `porcelain`
- `jiang`
- `day`
- `ink`
- `cartoon`
- `ledger`

Six share exactly the same `FORGE_CHROME`; most differentiation is color substitution. The gallery itself requires `web/src/pages/Themes.vue`, `web/src/themes/galleryMotion.ts`, `web/src/themes/MockKeyboard.vue`, routing state, transition handling, mock UI, persistence, and a large block of theme-specific CSS.

The declared typography is also aspirational. Theme stacks reference fonts including Big Shoulders Display, Martian Mono, Bagel Fat One, M PLUS Rounded 1c, Share Tech Mono, and IBM Plex Mono. No corresponding `@font-face` rules exist under `web/src`, and `SECURITY.md` says the dashboard loads no external fonts. On ordinary systems these themes collapse to fallbacks.

The public profile adds three more palettes—Cobalt, Magenta, and Newsprint—through a separate system in `internal/profilewebembed/static/assets/profile.js`.

Eleven variants across two unrelated systems are not product depth. Keep one light and one dark presentation.

### The public profile is a second analytics application with manual freshness

The public profile introduces:

- `internal/publicprofile/`
- `internal/profilewebembed/`
- `profile-e2e/`
- `docs/public-profile.schema.json`
- `docs/public-profile.md`
- `docs/media/public-profile-demo/`
- `public-profile/`
- preview SVG generation
- bundle manifests and asset revisions
- production provenance validation
- model and cost publication flags
- three activity palettes
- a legacy card renderer in `internal/card/`

The result is a manually generated, manually committed snapshot whose main visual is another 53-week contribution graph. It duplicates a familiar GitHub surface while being less current than the local dashboard by construction.

`internal/profilewebembed/static/assets/profile.js` applies an unexplained absolute saturation point:

```js
const ABSOLUTE_TOKEN_CAP = 1_000_000_000;
```

That constant determines visual intensity regardless of the user’s own range. It gives low-volume profiles little contrast and encodes a product judgment without validation.

The “legacy” `wheretoken card` compatibility path is especially weak. `README.md` labels it compatibility behavior despite the entire public-profile line being recent. Premature compatibility code is still dead weight.

Remove public profiles from v1. A JSON export is enough until there is demonstrated demand for a publication product.

### Newsprint is an asset pipeline looking for a reason to exist

The Newsprint treatment spans:

- `docs/wheretoken_newsprint_material_design_v3.md`
- `scripts/gennewsprint/`
- `scripts/gennewsprint/vendor/Paper001_Color.jpg`
- `scripts/gennewsprint/vendor/Paper001_Displacement.jpg`
- `scripts/gennewsprint/vendor/SOURCE.md`
- `scripts/gennewsprint/material-swatch.html`
- `internal/profilewebembed/static/assets/newsprint-surface.jpg`
- duplicated profile bundles
- CSS texture handling
- specialized SVG ink generation in `internal/publicprofile/preview.go`

`scripts/gennewsprint/vendor/SOURCE.md` records checksums for the upstream archive and maps, but not separate checksums for the transformed local working extracts or final output. Git objects preserve the committed files, but the provenance document does not fully describe the transformation from upstream inputs to those extracts.

The committed `public-profile/preview-light.svg` contains 161 `<pattern>` elements. That is substantial generated structure for a paper-and-ink effect inside a static preview.

The repository history described in `docs/v1-review/repository-map.md` shows repeated Newsprint redesigns: fold paths, procedural material, processed photoscan, then further preview treatment. This is visual churn, not v1 stabilization.

Delete the entire material pipeline. A flat color needs no provenance document, source maps, bake program, swatch, duplicated JPEG, or per-cell SVG patterns.

## Repository and architecture problems

### Generated ownership is unnecessarily indirect

The repository commits several kinds of generated or duplicated output:

- `internal/webembed/dist/`
- `web/public/sample/`
- `docs/media/public-profile-demo/`
- `public-profile/`
- duplicated Newsprint assets
- `.github/workflows/`
- `ci/github-workflows/`

The canonical workflows live under `ci/github-workflows/` and are copied into `.github/workflows/` by `scripts/install-github-workflows.sh`. GitHub executes the copies, not the declared source. That arrangement exists solely to create drift that another mechanism must detect.

Use `.github/workflows/` as the source of truth and delete the installer and duplicate tree.

### Release builds mutate the checkout

`.goreleaser.yaml` runs `go mod tidy` as a release hook. A release from a tag should verify that dependency metadata is already correct, not repair it in the release workspace.

Replace mutation with a read-only verification step and fail when `go.mod` or `go.sum` is untidy.

### The shared dashboard has too many runtime identities

`web/` supports local, demo, and hosted modes. The modes use different endpoints, privacy boundaries, authorities, controls, and authentication states.

Build flags reduce duplication but do not make these one product. Every dashboard change must be reasoned about across a localhost scanner, fabricated static demo, and authenticated hosted account.

If hosted is removed from v1, delete the hosted mode rather than retaining dormant conditionals.

### “Full” source claims are too absolute

The supported-agent table in `README.md` labels many adapters “Full,” while adjacent documentation says completeness varies by product, authentication, source window, and exposed fields.

“Full” has no defined contract. It suggests completeness across token categories, history, models, requests, and turns when adapters actually inherit the limitations of their source ledgers.

Replace “Full” with explicit capabilities and known window limits from `docs/data-sources.md`.

### Personal publication artifacts do not belong in the core repository

`public-profile/` is the maintainer’s production snapshot, while `docs/media/public-profile-demo/` is a synthetic demonstration and `internal/profilewebembed/static/` is the renderer source.

The core repository should not simultaneously act as product source, demo host, and personal analytics publication. It complicates privacy review and makes every renderer change produce unrelated artifact churn.

### History shows feature accumulation, not a stabilization phase

`docs/v1-review/repository-map.md` records pricing, portrait, hosted sync, public profiles, editorial redesign, profile palettes, multiple Newsprint rebuilds, and installer changes across a compressed release sequence.

The problem is not commit count. It is that high-level product identity was still changing while v1 maturity was supposedly under review. A maturity phase should be removing surfaces and closing correctness gaps, not inventing more presentation systems.

## Open-source maturity gaps

The repository has no:

- `CONTRIBUTING.md`
- `CODE_OF_CONDUCT.md`
- `SUPPORT.md`
- issue templates
- pull-request template

`SECURITY.md` is only a few paragraphs. “Report vulnerabilities privately to the GitHub owner” is not an actionable disclosure channel. It does not identify supported versions, GitHub private advisories, a security address, hosted-service scope, expected response handling, or data-deletion escalation.

The project distributes binaries, reads sensitive local application stores, accesses product APIs using existing credentials, and operates hosted authentication code. Its contribution and security process is less mature than its threat surface.

The unpublished `npm/` wrapper is another non-product. `README.md` advertises that it is not on npm, while the repository still carries its package metadata and tests. Delete it until publication is real.

## Design and taste issues — not release blockers

These do not independently block v1, but they expose the lack of product discipline:

- `README.md` says the dashboard has a 2×5 KPI readout, while `docs/media/dash-newspaper.jpg` depicts the older six-cell layout.
- The product mixes furnace language, a kiln mascot, glaze themes, ledger styling, newspaper styling, editorial public profiles, Cobalt/Magenta palettes, and a generic logo. This is several visual metaphors competing for ownership.
- The local dashboard is Chinese-first, theme marketing copy is Chinese, public-profile UI is English, and the primary README is English. The language strategy follows the surface rather than the user.
- “窑 is whereToken’s furnace mascot” in `README.md` explains an internal metaphor instead of helping users understand accounting.
- Theme descriptions such as “author’s favorite color scheme” in `web/src/themes/manifest.ts` are repository personality, not product documentation.
- The public profile’s contribution-wall resemblance is derivative. Newsprint texture does not make the underlying interaction distinct.
- The theme gallery animation and full-page mock preview in `web/src/pages/Themes.vue` receive more interaction design than source coverage and data-quality explanation.
- A hard-coded one-billion-token visual cap is presented as stable behavior in `docs/public-profile.md` without evidence that it produces readable profiles across real usage distributions.
- `wheretoken card` should not be preserved merely because a recently introduced command already exists.

## Ruthless deletion list

Delete or remove from the v1 distribution:

1. `cmd/wheretoken-hosted/`
2. `internal/hosted/`
3. `internal/syncagg/`
4. hosted branches and account UI under `web/`
5. CLI `login`, `logout`, and `sync`
6. hosted credential storage in `internal/credstore/` if no other feature needs it
7. `docs/architecture/hosted-web-v070.md`
8. `internal/community/`
9. `wheretoken community`
10. the community block in JSON output
11. `docs/community.md`
12. `internal/profile/`
13. portrait fields in report and dashboard contracts
14. `internal/insight/evaluation.go` and the unused dashboard `evaluation` payload
15. six of the eight dashboard themes
16. `web/src/pages/Themes.vue`
17. `web/src/themes/galleryMotion.ts`
18. `web/src/themes/MockKeyboard.vue`
19. the public-profile palette switcher
20. `internal/publicprofile/`
21. `internal/profilewebembed/`
22. `profile-e2e/`
23. `public-profile/`
24. `docs/media/public-profile-demo/`
25. `docs/public-profile.md`
26. `docs/public-profile.schema.json`
27. `internal/card/` and `wheretoken card`
28. `scripts/genprofiledemo/`
29. `scripts/genprofilecandidates/`
30. `docs/wheretoken_newsprint_material_design_v3.md`
31. `scripts/gennewsprint/`
32. every committed `newsprint-surface.jpg`
33. Newsprint-specific CSS and SVG pattern generation
34. `npm/` until an npm package is actually published
35. `ci/github-workflows/`
36. `scripts/install-github-workflows.sh`
37. stale screenshots and generated personal publication artifacts
38. the undefined “Full” labels in `README.md`
39. `go mod tidy` from the release hook
40. any v1 claim that includes a hosted service before its storage protocol is corrected and obligatorily tested

## Smallest credible v1 bar

A credible v1 should contain only:

1. **Local CLI accounting** from explicitly registered adapters.
2. **JSON output** with a documented, versioned schema.
3. **One loopback-only dashboard** reading the same local aggregation.
4. **One light and one dark visual treatment**, with bundled or honest system fonts.
5. **Explicit capability reporting** per adapter: token categories, requests, turns, model labels, history window, authentication requirement, and unavailable states.
6. **No hosted accounts, synchronization, leaderboard, portrait, public profile, card renderer, or material pipeline.**
7. **Mandatory CI** for token arithmetic, append-only scan invariants, adapter redaction, malformed inputs, missing-versus-zero behavior, dashboard tests, installers, and generated-build drift.
8. **Signed or independently verifiable releases**, with the release build failing on a dirty or untidy checkout.
9. **Accurate screenshots and privacy documentation** that enumerate every network-capable adapter and the exact reason it contacts an upstream API.
10. **Basic open-source operations:** contribution guide, actionable security disclosure route, supported-version policy, issue templates, and pull-request expectations.
11. **A frozen v1 schema and command set** with no compatibility aliases for experiments introduced immediately before release.
12. **Removal of stale plans and duplicate sources of truth** so a new maintainer can identify canonical code, workflows, assets, and documentation without reading repository archaeology.
