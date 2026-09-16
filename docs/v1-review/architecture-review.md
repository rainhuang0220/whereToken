# whereToken v1 Architecture Review

**Reviewer:** Reviewer A, independent senior open-source architecture maintainer  
**Implementation revision reviewed:** `de3dfb4`  
**Review context:** `docs/v1-review/repository-map.md` from documentation-only commit `8d71110`  
**Branch:** `cursor/v1-maturity-review-27f1`

## Executive verdict

whereToken has a strong local accounting kernel, unusually disciplined privacy rules, and a public-profile boundary that is better designed than many mature open-source analytics projects. The central architecture—source adapters producing normalized events, one metric engine, a disposable local index, explicit quality and derivation metadata, and privacy-safe projections—should be preserved.

The repository is not yet uniformly v1-mature.

The local CLI/dashboard path is close to v1 quality. Its token arithmetic, request deduplication, append-only scanning behavior, missing-vs-zero handling, pricing honesty, and cross-platform tests are deliberate and well defended.

The hosted path is materially less mature. Its high-level design is sound, but the current implementation has correctness and trust-boundary defects in revision handling, payload validation, aggregate reconstruction, status persistence, CSRF enforcement, rate limiting, and operational lifecycle. These do not justify a rewrite. They justify focused stabilization around the existing contracts.

My recommendation is:

- **Preserve and harden** the local accounting architecture.
- **Treat hosted synchronization as not yet v1-ready** until its conflict and validation semantics are corrected.
- **Do not replace normalized events, the metric engine, the local-first boundary, the disposable index model, or the public-profile allowlist.**
- Reduce duplicated projections and semantic copies only after the load-bearing invariants have explicit parity tests.

This is a maturity problem at the edges, not a failed core architecture.

## Scope and evidence standard

The review covered:

- adapter registration, discovery, parsing, and normalized events;
- scan orchestration and status representation;
- the SQLite index and append-only behavior;
- metric aggregation, windows, calendars, pricing, and drill-downs;
- CLI report projection;
- public-profile generation, validation, and static bundles;
- hosted identity, pairing, sync, storage, and dashboard reconstruction;
- frontend build modes;
- generated artifacts;
- test strategy, CI, release, Pages, and hosted deployment;
- recent repository history.

Claims below are marked as:

- **Evidence:** directly observable in source, tests, workflows, or revision history.
- **Assessment:** architectural judgment or a likely consequence derived from that evidence.

## Architectural model

The intended data flow is coherent:

```text
Local agent ledgers / product usage APIs
             |
             v
     adapter.Discover / Parse
             |
             v
   UsageEvent + TurnEvent
             |
        +----+-----+
        |          |
        v          v
  local index   metric.Aggregate
   (cache)           |
                  +--+-------------------+
                  |          |           |
                  v          v           v
               CLI report  dashboard  public profile
                                         |
                                         v
                               allowlisted static bundle

Local scan
   |
   v
syncagg daily-model projection
   |
   v
hosted MySQL projection
   |
   v
hosted dashboard reconstruction
```

The most important architectural distinction is correct: local agent data remains authoritative; the index is only a cache; hosted storage is a privacy-reduced projection; public profiles are an even narrower publication projection.

Evidence for this separation appears in:

- `internal/adapter/adapter.go`
- `internal/event/event.go`
- `internal/index/store.go`
- `internal/metric/summary.go`
- `internal/report/snapshot.go`
- `internal/syncagg/batch.go`
- `internal/publicprofile/snapshot.go`
- `internal/hosted/dashboard.go`
- `docs/token-accounting.md`
- `docs/data-sources.md`
- `docs/architecture/hosted-web-v070.md`

## What is genuinely well designed

### 1. Token accounting is treated as a product contract

**Evidence:** `docs/token-accounting.md` defines the four additive token components, explicitly excludes reasoning as a second output charge, documents provider mappings, and defines request merging. `internal/metric/summary.go` implements the same arithmetic with saturating addition and rejects events containing negative billed components. `internal/metric/adversarial_test.go` covers negative values, overflow, unknown pricing, duplicate requests, and undated events.

**Assessment:** This is the correct center of gravity. Token accounting is not hidden inside adapters or UI code. The repository has a reviewable definition of what “Total” means and tests adversarial cases rather than only examples.

### 2. The normalized event boundary is small and useful

**Evidence:** `internal/event/event.go` contains two compact structures: `UsageEvent` and `TurnEvent`. The adapter interface in `internal/adapter/adapter.go` only requires `ID`, `Discover`, and `Parse`.

**Assessment:** The event model successfully absorbs substantial source diversity without imposing a large generic schema. `Quality`, `Derivation`, `SkipRequest`, and the four token components encode the distinctions that actually affect accounting.

This model should evolve conservatively, but it should not be replaced with provider-specific data flowing through the rest of the application.

### 3. Adapter ownership is explicit

**Evidence:** Each source has its own package under `internal/adapter/<id>/`. `internal/adapter/catalog.go` owns labels and capabilities. `internal/scan/scan.go` owns instantiated adapters. `internal/adapter/contract_test.go` verifies that the catalog matches registration and checks empty discovery, malformed input, negative values, and planted-secret leakage.

`docs/adding-an-adapter.md` requires source-specific fixtures, malformed-row behavior, privacy tests, quality, derivation, and documented token mappings.

**Assessment:** Manual registration in two places is acceptable at this scale because it is explicit and contract-tested. It is preferable to filesystem or reflection-based registration that could make mere configuration directories appear to be usage sources.

### 4. The index has the correct authority model

**Evidence:** `internal/index/store.go` states that agent data remains authoritative and that the SQLite database is a disposable performance cache. It records file size, mtime, inode, and consumed offset. `LoadOrParse` incrementally parses append-only sources; `LoadOrReplay` fully reparses stateful or non-resumable sources.

On an incremental parse failure, `LoadOrParse` returns the old cached events with the error. `internal/index/bind_test.go` verifies that cached events are still emitted. `internal/index/jsonl.go` leaves an incomplete trailing record unconsumed and consumes malformed complete lines so scanning does not become permanently stuck.

**Assessment:** This is a strong design. The cache accelerates parsing without becoming an undocumented second ledger. Keeping cached events when an appended tail fails is exactly the right monotonic behavior for an accounting tool.

### 5. Append-only and reset-archive behavior is load-bearing and tested

**Evidence:** `docs/token-accounting.md` states that later successful scans must not drop tokens when files only append. `docs/data-sources.md` explicitly includes OpenClaw `.jsonl.reset.*` and `.jsonl.deleted.*` archives. Tests in `internal/index/store_test.go`, `internal/index/jsonl_test.go`, and source-specific adapter tests cover append parsing, partial tails, rewrites, and parse failures.

**Assessment:** This invariant is one of the project’s most valuable properties. It should be treated as accounting correctness, not merely index behavior.

### 6. Missing-vs-zero is modeled deliberately

**Evidence:**

- `event.QualityAbsent` and `event.QualityDegraded` are distinct in `internal/event/event.go`.
- `fillMissingSources` in `internal/scan/scan.go` creates absent or degraded source slices.
- `usageUnavailable` in `internal/report/snapshot.go` renders unavailable rows as unavailable rather than `0.00 M`.
- `internal/metric/summary.go` uses pricing status and omitted JSON cost fields so unknown prices do not become `$0`.
- `internal/publicprofile/snapshot.go` uses nullable values plus explicit component status.
- `internal/publicprofile/production.go` rejects production profiles with incomplete cloud token coverage unless explicitly overridden.

**Assessment:** The project understands that a numeric Go zero and a measured zero are not the same thing. That distinction is visible in code, documentation, tests, and UI behavior rather than existing only as prose.

### 7. Pricing has one accountable source of truth

**Evidence:** Rates live in `internal/price/table.go`; official URLs and verification dates live in `internal/price/catalog.go`. `metric.AggregateAt`, CLI pricing, model views, public profiles, and hosted reconstruction all reuse the Go price package.

Unknown component rates remain unavailable. A listed free component requires an explicit marker rather than inferring free from a zero-valued rate.

**Assessment:** This is excellent. Copying rates into JavaScript or SQL would create silent financial fiction; the current architecture avoids that.

### 8. Public-profile generation is an exemplary publication boundary

**Evidence:**

- `internal/publicprofile/snapshot.go` defines a dedicated allowlisted contract and explicitly states that renderers consume `Snapshot`, not scan results or raw events.
- `internal/publicprofile/allowlist.go` maps unknown sources, vendors, and models to bounded public values.
- `internal/publicprofile/validate.go` combines JSON Schema validation, semantic reconciliation, URL checks, sensitive-string scanning, and a compressed-size budget.
- `internal/publicprofile/production.go` adds production-specific provenance and cloud-coverage gates.
- `internal/publicprofile/bundle.go` produces a manifest, content-derived asset revision, previews, and a fixed generated-file list.
- `internal/publicprofile/publication_test.go` ensures synthetic demo data cannot be published as a production profile.
- `internal/publicprofile/validate_schema_test.go` rejects additional properties throughout the schema and verifies semantic tampering.
- `internal/publicprofile/bundle_test.go` verifies provenance, snapshot identity, asset digests, retired-file removal, and exact reuse of embedded assets.

**Assessment:** This is the strongest subsystem in the repository. Its defense is structural: a narrow DTO, strict schema, semantic checks, allowlisting, provenance, and publication gates. The denylist is only a final backstop, not the primary privacy mechanism.

### 9. Local and hosted servers are separated correctly

**Evidence:** `internal/httpapi/httpapi.go` refuses non-loopback binding and enforces Host and Origin/Referer checks. Its tests cover foreign Host, foreign Origin, SPA fallback, safe headers, and local-only behavior.

The hosted mux in `internal/hosted/http.go` does not register local scan routes. `internal/hosted/http_test.go` explicitly verifies that `/api/scan` is absent.

**Assessment:** This is a load-bearing security boundary. A public server must never reuse a mux capable of opening local agent files. The repository makes that separation explicit.

### 10. Hosted synchronization chooses the right privacy shape

**Evidence:** `internal/syncagg/batch.go` uploads daily-by-model aggregates rather than events. `internal/syncagg/batch_test.go` constructs events containing paths, session IDs, request IDs, and secret-looking strings, then verifies that none enter serialized sync JSON.

`internal/syncagg/scope.go` distinguishes Cursor account-global provider rows from device-local ledgers. Source identifiers are HMAC-derived through the paired account key rather than uploading local paths or enumerable user identifiers.

**Assessment:** The hosted data model is conceptually correct. Daily-model aggregates are sufficient for dashboard reconstruction while avoiding prompts, raw transcripts, workspaces, and session identifiers. The defects are in implementation details, not in this privacy decision.

### 11. Generated public assets have unusually good provenance controls

**Evidence:** Public-profile source assets live under `internal/profilewebembed/static`. Generated bundles under `docs/media/public-profile-demo` and `public-profile` are tested for exact equality with those embedded assets. `bundleAssetRevision` hashes presentation assets independently from the snapshot ID.

The Pages workflow checks provenance before publishing `public-profile`, and publishes the synthetic demo separately under `_pages/profile-demo`.

**Assessment:** Separating data identity from asset identity is a good decision. A stylesheet-only change invalidates presentation caches without pretending the usage snapshot changed.

### 12. CI is broad and dependency actions are pinned

**Evidence:** `.github/workflows/ci.yml` runs formatting, vet, Go tests, Go builds, frontend tests, npm wrapper tests, CLI fixtures, vulnerability checks, and selected race tests across Linux, macOS, and Windows. Public-profile browser tests run on Linux. Action dependencies are pinned to commit SHAs.

At `de3dfb4`, the Linux, macOS, Windows, Pages build, and Pages deploy checks all completed successfully in [CI run 35140826268](https://github.com/rainhuang0220/whereToken/actions/runs/35140826268) and [Pages run 35140826249](https://github.com/rainhuang0220/whereToken/actions/runs/35140826249).

**Assessment:** The project tests actual product surfaces, including installer shell lifecycle and generated publication boundaries. That is stronger than a conventional package-only Go test suite.

## Highest-risk findings

### P0: Hosted revision semantics can acknowledge stale data

**Evidence:**

- The CLI assigns every row `Revision: a.Now().Unix()` in `internal/cli/sync.go`, giving one-second resolution.
- Account-global rows are stored with canonical `device_id = 0` in `internal/hosted/store.go`, so different devices compete using independently generated wall-clock revisions.
- For an equal revision, `ApplyUsage` selects only `revision` and `miss`. It reports a conflict only when `miss` differs. If `cache_read`, `cache_create`, `output`, `requests`, `user_turns`, `quality`, or `derivation` differs while `miss` is unchanged, it silently accepts the request without updating the row.
- `internal/hosted/store_test.go` tests stale revisions and idempotent retries, but not full-row equality on equal revisions or skewed multi-device account-global writes.

**Assessment:** Two syncs within one second can use different idempotency keys but the same revision. An output-only change can receive HTTP 200 while the hosted row remains stale. A miss change receives a conflict even though it may be a legitimate newer scan.

For account-global rows, device clock skew is more serious: one device with a future clock can cause valid writes from another device to be rejected as stale. Wall-clock seconds are not a valid monotonic revision shared by independent writers.

This is a v1 blocker if the hosted dashboard is presented as a faithful projection of local accounting.

### P0: Hosted batch validation is insufficient for a public trust boundary

**Evidence:** `syncagg.DecodeBatch` in `internal/syncagg/batch.go` validates schema version, device ID, source scope, and model length. It does not validate:

- nonnegative token and count fields;
- upper bounds on counts;
- positive revisions;
- date syntax and range;
- tool, vendor, hash, status, quality, or derivation bounds;
- generated timestamp or timezone semantics.

`internal/hosted/sync.go` limits body size, row count, and source count, but not the values inside each row.

`aggregateRows` in `internal/hosted/dashboard.go` expands `row.UserTurns` into one synthetic `TurnEvent` per turn.

**Assessment:** An authenticated but malicious or compromised device can store negative or extreme values. A row with a very large `user_turns` value can make a dashboard request allocate or loop over an enormous synthetic event set. The bearer requirement reduces exposure but does not make client input trusted.

The server must defend its own accounting and resource bounds independently of the official CLI.

### P0: Local and hosted canonicalization have already drifted

**Evidence:** There are two independent `mergeByRequest` implementations:

- `internal/metric/summary.go`
- `internal/syncagg/batch.go`

The metric implementation:

- rejects negative billed components;
- keeps per-component maxima;
- keeps the maximum reasoning value;
- promotes quality by rank;
- assigns the canonical event to the latest timestamp.

The sync implementation:

- does not reject negative values;
- keeps per-component maxima;
- leaves the first timestamp;
- only fills quality and derivation when initially empty;
- applies different `SkipRequest` behavior.

Hosted reconstruction in `internal/hosted/dashboard.go` creates all synthetic usage events with `SkipRequest: true`, then manually repairs request counts only for `Summary.All` and `BySource`. `ByVendor`, `ByModel`, and `BySourceVendor` do not receive equivalent request counts. The hosted payload also omits some fields required by the frontend `SummaryPayload` type in `web/src/types.ts`, including `by_source_vendor`.

**Assessment:** The privacy projection no longer has exactly the same semantics as the local metric engine. A streamed request crossing a local-day boundary can be assigned to a different date after sync. Quality may differ, and hosted request breakdowns can disagree with local views.

The duplicate canonicalization is not merely a future drift risk; semantic drift is present at this revision.

### P0: Cookie-authenticated mutations do not require the documented CSRF header

**Evidence:** `requireCSRF` in `internal/hosted/auth.go` reads `X-CSRF-Token`, but if the header is absent it falls back to the `wt_csrf` cookie. Session and CSRF cookies use `SameSite=Lax`.

The architecture document says mutating routes require a header matching the session-bound token: `docs/architecture/hosted-web-v070.md`.

**Assessment:** SameSite is based on site, not origin. A compromised sibling under `plainlist.space` can send same-site requests with whereToken cookies. Because the server accepts the cookie without the header, it loses the intended proof that same-origin JavaScript read the CSRF token. Several handlers also decode JSON without requiring a non-simple content type.

This is a concrete boundary mismatch, especially because the documented deployment hosts several sibling applications.

### P0/P1: Hosted rate limiting collapses behind the documented reverse proxy

**Evidence:** `limit` in `internal/hosted/pair.go` derives its key from `r.RemoteAddr`. `docs/deployment.md` and `docs/architecture/hosted-web-v070.md` specify nginx proxying `/api/` to `127.0.0.1:3400`.

**Assessment:** At the Go process, requests arrive from nginx, so clients share the proxy address. The advertised per-IP limits become service-wide limits. A small amount of traffic can throttle unrelated users, while the limiter provides little discrimination against the original client.

The map also retains keys indefinitely after their callers stop returning, producing slow unbounded growth.

### P1: Sync metadata is best-effort while the UI treats it as authoritative

**Evidence:**

- `Store.SyncBatch` commits usage rows first, then updates timezone and `source_states` outside that transaction.
- Errors from timezone and source-state updates are discarded in `internal/hosted/store.go`.
- `putSyncBatch` discards errors from `TouchSync` in `internal/hosted/sync.go`.
- `getDashboard` discards errors from `ListDevices` and defaults timezone failures in `internal/hosted/dashboard.go`.
- `source_states` is written and deleted but never read anywhere in production code.
- `hostedMeta` derives `last_sync_at` from devices, and `web/src/hosted/state.ts` uses that metadata to report synced or stale state.

**Assessment:** Accounting rows can commit while last-sync metadata remains stale. The HTTP request still returns success. The frontend may claim a synchronized state without reliable metadata, and source status cannot support the status strip described in the hosted ADR because it is currently write-only.

### P1: Pairing and OAuth consumption are not atomic

**Evidence:** Pair confirmation in `internal/hosted/pair.go` performs:

1. `GetPairChallenge`;
2. `InsertDevice`;
3. `ConsumePair`;
4. insertion of the raw token into an in-memory `sync.Map`.

The challenge read, device creation, and consumption are not one transaction. `ConsumePair` does not check affected rows. OAuth state consumption in `internal/hosted/store.go` similarly selects and deletes in separate statements.

**Assessment:** Concurrent confirmation requests can race and create multiple devices for one challenge. A process restart after challenge consumption but before the CLI polls loses the only raw device token, leaving the CLI with `consumed` and requiring re-pairing. Concurrent OAuth callbacks may both pass the state read before either deletion completes.

These are contained failures, but they show that hosted one-time-token lifecycle is not yet strongly atomic.

### P1: The index identity test exposes both a known limitation and test flakiness

**Evidence:** `internal/index/store.go` explicitly describes `(path, size, mtime, inode)` as a best-effort identity and states that a same-size rewrite restoring old metadata may replay stale blobs.

`TestReplaceSameSizeDifferentInodeForcesFull` in `internal/index/store_test.go` removes and recreates a file but only checks whether the resulting inode is nonzero; it does not verify that the new inode differs from the old one.

In this environment, repeated execution failed four out of five times with `mode=unchanged`. That mode requires size, mtime, and inode to match the cached tuple.

The exact revision nevertheless passed Linux, macOS, and Windows CI.

**Assessment:** The local failure is primarily a nondeterministic test premise: filesystems may immediately reuse a deleted inode, and timestamp resolution can preserve the same mtime. It should not be presented as proof that inode-change detection is broken.

It does, however, demonstrate the documented cache limitation. A metadata-identical replacement can replay stale cached events until rebuild. This is acceptable only while the index remains explicitly disposable and accounting documentation remains honest about it.

### P1: Missing-vs-zero remains a distributed convention

**Evidence:** `fillMissingSources` creates a `metric.Slice` with zero numeric fields plus `QualityAbsent`. `metric.View` still formats those numbers normally. `report.rowFrom`, frontend formatting, and public-profile projection separately interpret quality to render unavailable.

Hosted tables use non-null numeric columns plus quality/status side data, while `source_states` is not read.

**Assessment:** The behavior is currently correct on tested product surfaces, but the type system does not prevent a new consumer from rendering an absent source as zero. Public profiles solve this more robustly with explicit status and nullable values. Other projections rely on every renderer remembering the convention.

This is technical debt around a load-bearing invariant, not permission to replace the current behavior.

### P1: Local-only metadata is not type-isolated

**Evidence:** `UsageEvent` contains `SourceRoot`, `Workspace`, `SessionID`, and `RequestID`; `TurnEvent` contains workspace and session values. `adapter.SourceRoot` contains local paths and optional auth paths.

Sync privacy tests prove those fields are omitted by `syncagg.Batch`, and public profiles consume a dedicated allowlisted snapshot.

Errors are accumulated raw in `scan.Result.Errors` and redacted later by `internal/report/redact.go` in report and JSON presentation paths.

**Assessment:** Local workspaces and session identifiers are useful for drill-downs, so their existence is understandable. The risk is that `UsageEvent`, `SourceRoot`, and `scan.Result` are unsafe-to-publish structures without type-level enforcement. A future serializer or logging call could cross the privacy boundary accidentally.

The current projections defend the boundary well, but the core types should be treated as sensitive local-domain objects.

### P1: Hosted operations are not production-hardened

**Evidence:**

- `cmd/wheretoken-hosted/main.go` sets only `ReadHeaderTimeout`.
- It has no signal-driven graceful shutdown.
- `/api/health` in `internal/hosted/http.go` returns `status: ok` without calling `Store.Ping`, despite `Store.Ping` existing.
- MySQL migrations in `internal/hosted/migrate.go` are an append-only list of DDL statements without a migration ledger or foreign keys.
- Hosted tests start `mysql:5.7` in `internal/hosted/mysqltest.go`.
- MySQL 5.7 is end-of-life.
- `scripts/build-hosted.sh` builds the binary and hosted SPA, but no committed deployment definition controls nginx, process supervision, migrations, or rollback.

**Assessment:** The hosted service can appear healthy while its database is unavailable. Abrupt process replacement can interrupt requests. Schema evolution is manageable while the service is small but will become risky once user data has lasting value.

The documented reverse-proxy deployment is reasonable, but the operational contract is mostly prose and manual state.

### P2: Previous-window comparison is DST-sensitive

**Evidence:** `metric.ParseWindow` uses calendar-aware `AddDate`, but `Window.Previous` in `internal/metric/window.go` computes a duration with `To.Sub(From)` and subtracts that duration.

**Assessment:** A window crossing a daylight-saving transition may be 167 or 169 hours. Subtracting that duration can shift the preceding wall-clock boundary by one hour. Calendar totals remain robust because they use local dates, but comparison windows can be subtly inconsistent.

## Accidental complexity and technical debt

### 1. Too many presentation projections

The same accounting result is represented as:

- `metric.Slice` and `metric.Summary` in `internal/metric/summary.go`;
- `metric.SliceView`;
- scan’s private `summaryJSON` and `sourceVendorView` in `internal/scan/scan.go`;
- `report.Snapshot` and `report.Row` in `internal/report/snapshot.go`;
- public-profile `Snapshot`, `Period`, `Component`, and `Breakdown`;
- sync `Batch` and `DailyModel`;
- hosted synthetic events;
- TypeScript copies in `web/src/types.ts`.

Some duplication is justified because local, hosted, CLI, and public surfaces have different privacy requirements. The accidental part is duplication of accounting semantics—especially request merging and hosted request reconstruction—rather than duplication of serialization shape.

### 2. `scan` owns too many responsibilities

`internal/scan/scan.go` currently owns:

- adapter registration;
- cache setup;
- multi-home orchestration;
- filesystem duplicate suppression;
- progress reporting;
- raw error accumulation;
- metric aggregation;
- missing-source classification;
- windows and comparisons;
- JSON presentation;
- redaction calls;
- profile and insight evaluation;
- community projection.

This package has become the integration center for both execution and presentation. The dependency on `internal/report` solely for redaction is a visible boundary leak.

It should not be rewritten wholesale. Its responsibilities should be reduced only after behavior is pinned by parity tests.

### 3. Hosted reconstruction has projection impedance

`internal/hosted/dashboard.go` converts database aggregates back into synthetic events and turns to reuse the metric engine. Reusing `metric` is the correct principle. Creating one `TurnEvent` per stored count and manually repairing request fields are signs that the current metric API is event-shaped while hosted storage is aggregate-shaped.

This is a narrow architectural mismatch, not a reason to duplicate the entire metric engine in SQL.

### 4. Generated artifacts have several ownership modes

There are three distinct generated systems:

- `web/` source compiled into `internal/webembed/dist`;
- fabricated dashboard samples under `web/public/sample`;
- public-profile source assets under `internal/profilewebembed/static`, copied into `docs/media/public-profile-demo` and `public-profile`.

The public-profile system has strong exact-copy and manifest tests. The dashboard embed has lighter checks: `internal/webembed/fs_test.go` verifies presence and offline font behavior, but not exact reproduction from the current frontend source.

Release builds regenerate the dashboard embed through `.goreleaser.yaml`, while `scripts/package-release.sh` temporarily replaces and restores it. This works but is not immediately obvious to contributors.

### 5. Workflow files have canonical and installed copies

Canonical workflows live under `ci/github-workflows`, while active copies live under `.github/workflows`. `scripts/install-github-workflows.sh` copies them.

At the reviewed revision, all three pairs are byte-identical. Tests also inspect both Pages files for publication boundaries.

This is managed duplication, but still a maintenance hazard because GitHub only executes one copy. Drift prevention should remain automated.

### 6. Release channels are partly manual

`docs/releasing.md` documents manual post-release updates for the in-repo Homebrew formula, the external tap, npm metadata, public profiles, and dogfooding. Recent history includes separate distribution-fix commits after releases.

The documentation is honest, but the number of mutable release surfaces creates avoidable lag and version inconsistency risk.

### 7. The hosted ADR is stale relative to implementation

`docs/architecture/hosted-web-v070.md` remains marked “Proposed” and contains behavior not fully present in code, including strict header-based CSRF, per-IP rate limiting, source-state dashboard metadata, and stronger equal-revision comparison.

An ADR may document intent, but once the feature ships it must clearly distinguish accepted architecture from unimplemented requirements.

## Explicit non-rewrite list

“Do not rewrite” means preserve these contracts and semantics even if internal code is later reorganized.

1. **Do not replace local ledgers with the index or hosted database as accounting authority.**  
   Preserve the authority statement in `internal/index/store.go` and `docs/token-accounting.md`.

2. **Do not replace the normalized event boundary with provider-specific objects outside adapters.**  
   Preserve `UsageEvent`, `TurnEvent`, quality, derivation, and source-specific parsing under `internal/adapter`.

3. **Do not change Total to include Reasoning.**  
   Preserve `Miss + CacheRead + CacheCreate + Output` from `docs/token-accounting.md`.

4. **Do not replace max-per-component request canonicalization with last-row-wins or summation.**  
   Preserve the behavior in `internal/metric/summary.go`.

5. **Do not weaken append-only monotonicity.**  
   Incremental parse failures must continue emitting cached events, and reset/deleted transcript archives must remain visible.

6. **Do not turn missing usage or missing prices into numeric zero.**  
   Preserve `QualityAbsent`, explicit unavailable states, nullable public values, and omitted unknown cost.

7. **Do not move pricing into JavaScript, SQL, or adapter-local estimates.**  
   Preserve `internal/price/table.go` and `internal/price/catalog.go` as the single public-list-price card.

8. **Do not expose the local scanner through the hosted mux or a public bind.**  
   Preserve loopback restrictions and Host/Origin checks in `internal/httpapi`.

9. **Do not upload raw events, paths, workspaces, sessions, requests, prompts, or transcripts.**  
   Preserve the aggregate-only contract in `internal/syncagg` and its privacy tests.

10. **Do not replace account-global/device-local source scope with frontend deduplication.**  
    Source scope is an accounting rule and belongs before hosted storage.

11. **Do not replace public-profile allowlisting with “serialize and redact.”**  
    Preserve the dedicated `publicprofile.Snapshot`, strict schema, semantic validation, provenance, and production gate.

12. **Do not merge synthetic demo and production profile publication paths.**  
    Preserve the separation enforced by `internal/publicprofile/publication_test.go` and `.github/workflows/pages.yml`.

13. **Do not make the user portrait depend on prompts, usernames, hostnames, network identity, an LLM, or rank.**  
    Preserve deterministic evaluation in `internal/profile`.

14. **Do not remove source-specific fixtures and adversarial tests in favor of only interface mocks.**  
    The parser edge cases are the product.

## Test strategy assessment

### Strong coverage

The test suite is broad and generally aligned with architectural risk:

- adapter contract and source fixtures;
- malformed JSON and secret leakage;
- incremental index behavior and cache recovery;
- adversarial metric inputs;
- unknown and partial pricing;
- local HTTP host/origin restrictions;
- sync privacy projection;
- MySQL-backed hosted auth, pairing, sync, deletion, and deduplication;
- strict public-profile schema and publication provenance;
- frontend local/demo/hosted modes;
- browser testing;
- installer same-shell behavior;
- CLI rendering fixtures;
- vulnerability scanning.

Local verification during this review produced:

- `go vet ./...`: passed;
- core metric, scan, sync, public-profile, and non-skipped hosted tests: passed;
- public-profile and embedded-profile tests uncached: passed;
- frontend tests: 21 files and 167 tests passed;
- npm wrapper tests: 7 passed;
- the index replacement test remained locally flaky, failing four of five repetitions;
- Playwright was not run locally because the package had not been installed in `profile-e2e`; the exact revision’s Linux CI did run and pass the browser job.

### Gaps

#### Hosted MySQL tests can silently skip

`internal/hosted/mysqltest.go` skips integration tests when Docker or a test DSN is unavailable unless `WHERETOKEN_REQUIRE_MYSQL` is set. `.github/workflows/ci.yml` does not set that variable.

GitHub’s Linux runner normally supplies Docker, so the target CI likely exercised MySQL. However, a Docker regression could cause the hosted integration suite to skip while CI remains green.

#### Race testing misses stateful core packages

The race job does not include:

- `internal/index`
- `internal/scan`
- `internal/hosted`
- `internal/syncagg`
- `internal/publicprofile`

This is notable because scan/index use shared mutable seams and hosted has in-memory pending tokens and rate-limiter maps.

#### Release publication is not gated inside the release workflow

`.github/workflows/release.yml` publishes directly on a `v*` tag. Goreleaser rebuilds the frontend but does not run the documented Go, vet, frontend, browser, or CLI gates.

The release process relies on maintainers tagging a commit whose prior CI passed. That is a procedural dependency, not a workflow-enforced one.

#### The hosted artifact path is not exercised by CI

`scripts/build-hosted.sh` contains useful checks preventing demo samples from entering the hosted SPA. The regular CI workflow does not run this script. Pages builds demo mode; Goreleaser builds the local dashboard mode; neither proves the deployable hosted bundle.

#### Flake detection is absent

The index identity test passes on standard CI but is nondeterministic on a filesystem that reuses inode and timestamp metadata. There is no repeated-test or targeted flake job for this load-bearing cache behavior.

## CI/CD and deployment assessment

### CI

The cross-platform matrix and pinned Actions are good. Least-privilege permissions are used in CI, and Pages has only the permissions it needs.

The CI workflow is more mature than the hosted deployment path.

### Release

`.goreleaser.yaml` provides six platform/architecture archives, checksums, packages, completions, optional macOS signing, and embedded frontend rebuilding. `docs/releasing.md` clearly states which channels are manual and warns against inventing checksums or shipping after failed gates.

The primary risk is that the tag workflow itself does not enforce those gates.

### Pages

`.github/workflows/pages.yml` correctly separates:

- the static site;
- dashboard demo mode;
- synthetic public-profile demo;
- optional maintainer production profile.

Production profile provenance and snapshot identity are checked before copying. This is well designed.

### Hosted deployment

`scripts/build-hosted.sh` makes a correct distinction between `VITE_HOSTED=1` and demo mode and fails if sample artifacts leak into the hosted build.

Deployment remains manually specified through `docs/deployment.md` and the hosted ADR. There is no committed nginx or process-supervisor definition, no automated readiness check, and no maintained-database requirement. That is acceptable for an early single-host service, but not yet a robust v1 operational story.

## Recent-history assessment

Recent history shows two patterns.

### Positive pattern: regressions become explicit invariants

Examples include:

- `4c1f338` — keep cached events on incremental parse failure;
- `9b9aa10` — do not present missing usage as zero;
- `b549e44` — full-rescan same-size rewrites and cap huge lines;
- `36bc15f` — reject incomplete Cursor account pages;
- `e0cb001` — add cross-browser and publication gates;
- `292cf87`, `969f17b`, `44fdda1` — successively harden same-shell Windows installation;
- `abf56f3` — document installer invariants.

**Assessment:** The maintenance culture responds to defects by extracting architectural rules and tests. That is a strong predictor of long-term maintainability.

### Risk pattern: large surfaces landed in concentrated bursts

The hosted system was introduced across a short sequence:

- `0b847d4` — sync contract;
- `550c73d` — hosted store and binary;
- `141be6f` — OAuth and pairing;
- `97aab69` — CLI login and sync;
- `5bd66ad` — hosted dashboard;
- subsequent auth, schema, and pairing fixes through `9e6bb15`.

Public profiles similarly arrived and were followed by publication, coverage, and extensive presentation fixes. From `4eff0e8` through `de3dfb4`, multiple commits revised the same activity and Newsprint assets.

**Assessment:** Rapid feature delivery was followed by useful hardening, but hosted correctness did not receive the same depth of adversarial testing as local accounting and public publication. The concentrated profile churn is mostly presentation work, yet it increases generated-artifact maintenance pressure.

The history argues for stabilization, not redesign.

## Prioritized recommendations

These are outcome priorities, not an implementation plan.

### P0 — required before an end-to-end v1 claim

1. **Make hosted revision and conflict semantics deterministic across retries and devices.**  
   Equal revisions must compare the complete stored measure, and account-global writers must not depend on unrelated device clocks.

2. **Make the hosted API a strict validation boundary.**  
   Reject negative, malformed, and unreasonably large aggregate fields before storage or reconstruction.

3. **Restore exact local/hosted accounting parity.**  
   Canonical request merge, date assignment, quality, request counts, model/vendor breakdowns, and missing-vs-zero behavior must match one defined contract.

4. **Enforce CSRF according to the documented header-based contract.**  
   Cookie fallback must not make sibling-origin requests sufficient for mutation.

5. **Make rate limiting correct for the deployed proxy topology.**  
   Limits must discriminate intended clients without collapsing all nginx traffic into one bucket.

6. **Require hosted integration and hosted artifact tests in CI.**  
   A green build must prove that MySQL tests ran rather than skipped and that the deployable hosted SPA is not a demo build.

### P1 — v1 hardening

7. **Make synchronization metadata trustworthy.**  
   A successful response must not silently discard source status or last-sync failures. Exposed status should correspond to persisted state.

8. **Make one-time auth and pairing transitions atomic.**  
   Concurrent callbacks and confirmations should not create duplicate sessions/devices or lose the only token after a successful confirmation.

9. **Turn health into readiness and add normal server lifecycle controls.**  
   Production health should reflect required dependencies, and process replacement should allow in-flight work to drain.

10. **Adopt a maintained production database baseline and explicit migration history.**  
    MySQL 5.7 should not remain the long-term v1 foundation.

11. **Stabilize the index identity regression test.**  
    The test must prove the metadata condition it names, while the documented best-effort cache limitation remains explicit.

12. **Give unavailable data a harder boundary contract.**  
    New consumers should not need undocumented knowledge that zero plus `QualityAbsent` means unavailable.

13. **Treat local event and scan structures as sensitive types.**  
    Their path and session fields must remain structurally separated from public and hosted serialization contracts.

### P2 — maintainability

14. **Consolidate canonical accounting semantics.**  
    Request merging and aggregate reconstruction should have one authoritative behavior with parity tests across local, sync, hosted, report, and public-profile paths.

15. **Reduce `scan` package responsibilities without changing behavior.**  
    Registration, orchestration, privacy-safe presentation, and optional community decoration should become clearer boundaries over time.

16. **Strengthen generated-artifact provenance for the dashboard embed.**  
    Contributors should be able to prove whether committed embedded assets correspond to current frontend source.

17. **Keep workflow copies mechanically synchronized.**  
    The canonical/installed workflow arrangement is acceptable only while byte-level drift is continuously rejected.

18. **Align ADR status with shipped reality.**  
    Implemented, deferred, and violated hosted requirements should be distinguishable.

19. **Remove DST ambiguity from comparison semantics.**  
    Previous periods should be calendar-consistent in the selected local timezone.

20. **Reduce manual release-channel drift.**  
    Versioned artifacts, formulas, wrappers, and hosted deployment should have clearer machine-checkable relationships.

## Final assessment

whereToken’s core is not over-engineered. Its complexity largely reflects genuine differences among local ledgers, cloud account APIs, cache semantics, privacy projections, and product surfaces.

The architecture succeeds where it has one explicit source of truth:

- token math in `internal/metric`;
- price data in `internal/price`;
- adapter registration in `adapter.Catalog` plus `scan.Adapters`;
- publication schema in `internal/publicprofile`;
- local authority in agent ledgers rather than the cache.

It becomes risky where the same semantics are recreated:

- canonical merging in both metric and sync;
- aggregate reconstruction in hosted;
- unavailable-state interpretation in several renderers;
- workflow and generated-asset copies;
- procedural release and deployment state.

The correct v1 strategy is therefore conservative: preserve the local accounting kernel and privacy boundaries, then remove ambiguity from hosted synchronization and operational gates. A broad rewrite would put the project’s strongest properties at risk while failing to address the actual defects.
