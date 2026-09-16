# whereToken v1 decision

Date: 2026-09-16  
Decision owner: chief maintainer  
Inputs:

- `repository-map.md`
- `architecture-review.md`
- `design-review.md`
- `adversarial-review.md`
- baseline test, build, and browser evidence

## Executive decision

whereToken already has a credible core and a recognizable product identity.
The v1 problem is not lack of features. It is that the trust level varies by
surface:

- local accounting and the public-profile publication boundary are mature;
- the dashboard identity is distinctive and visually coherent;
- hosted synchronization has correctness and operational gaps that do not
  meet the standard set by the local product;
- public documentation does not explain the hosted network path with the same
  precision used for Community Rank and public profiles;
- several quality checks exist as conventions but are not mandatory gates.

The v1 strategy is therefore **stabilize and clarify**.

We will not rewrite the accounting kernel. We will not delete the public
profile, because a publishable coding-activity identity is central to the
stated product direction. We will not delete hosted sync merely because it is
complex; its aggregate-only architecture is sound and it is already shipped.
We will, however, treat hosted sync as below the v1 bar until its P0
correctness and trust-boundary defects are fixed.

The visual strategy is also conservative. The kiln dashboard is the primary
product identity. The public profile is intentionally more editorial and
GitHub-adjacent; it does not need a literal mascot pasted into it. Newsprint
currently uses real scanned paper, restrained processing, and no fake crease
paths. It is accepted for v1. The superseded procedural-height-field
specification is not a roadmap.

## How the reviews were resolved

The reviews agreed on the strongest facts:

1. normalized events and one metric engine are the right architecture;
2. missing usage must remain unavailable rather than numeric zero;
3. the index must remain a disposable cache;
4. the kiln wall, mascot, and honest-accounting language are memorable;
5. hosted sync has concrete correctness and lifecycle defects;
6. public privacy documentation omits a shipped network path;
7. CI does not prove every check that a v1 claim depends on;
8. recent Newsprint churn should stop.

They disagreed on product scope.

The hostile review proposed reducing v1 to CLI, JSON, and the loopback
dashboard. That would simplify the repository, but it would also abandon the
mission to become a developer identity product. The architecture review
showed that the public profile is not a casual serializer: it has an
allowlisted DTO, strict schema, semantic validation, provenance, and
production gates. That subsystem earns its complexity and stays.

The hosted path also stays, but with a stricter condition. It is an optional
aggregate projection, not the source of local truth. A v1 claim that includes
it must first close the verified accounting, validation, CSRF, pairing, and
test-gate defects.

The design review initially compared Newsprint to a superseded procedural
specification. That recommendation is rejected. Previous synthetic
height-field-only attempts were visibly unsuccessful. The current material
was judged in-browser to be real, matte, legible, seamless, and restrained.
More cockling, wrinkles, lighting, noise, or effects are not automatically
improvements.

## Keep

### Accounting kernel

- `UsageEvent` and `TurnEvent` as the normalized adapter boundary.
- Provider-specific parsing inside `internal/adapter/<id>`.
- Explicit tool registration through `adapter.Catalog` and `scan.Adapters`.
- Max-per-component request canonicalization.
- `Total = Miss + Cache Read + Cache Create + Output`.
- Reasoning as explanatory data, never a second output charge.
- Saturating arithmetic and rejection of negative local token events.
- One pricing table plus separately recorded official provenance.
- Model normalization for matching while preserving raw model identifiers.

### Local authority and privacy

- Agent ledgers as source of truth.
- The SQLite index as a disposable accelerator.
- Append-only monotonicity and retention of cached events after incremental
  parse failures.
- Reset/deleted transcript archives remaining visible.
- Loopback-only local HTTP with Host and Origin checks.
- Sensitive local event fields remaining unavailable to public projections.

### Public identity

- The sanitized public-profile snapshot as a first-class product surface.
- The allowlisted schema, semantic validation, production coverage gate, and
  explicit manual publication model.
- Separate synthetic-demo and maintainer-publication provenance.
- Separate snapshot and renderer asset identities.
- The compact editorial hierarchy: total, 53-week wall, trend, breakdown,
  technical details.
- GitHub-adjacent density without copying GitHub chrome.

### Product identity

- The kiln wall as the dominant dashboard visual.
- 窑 as a restrained status-bearing mascot.
- Ember, copper, clay, and firing vocabulary where it improves comprehension.
- The 2x5 KPI readout.
- Honest phrases such as “unavailable is not zero” and “public-list estimate,
  not a bill.”
- Existing dashboard themes. They will be frozen rather than expanded.

### Newsprint

- The ambientCG Paper001 CC0 source and recorded provenance.
- Real paper material, restrained retinting, static delivery, and excellent
  legibility.
- No fold overlay, no crease paths, no runtime shader, and no procedural noise
  as the main realism source.
- The current asset budget and browser-stable implementation.

## Change

### Hosted accounting and trust boundary

- Compare every persisted measure on equal revision, not only `miss`.
- Make idempotency cover the complete request envelope.
- Make usage, timezone, source status, and idempotency persistence atomic.
- Validate every client-controlled aggregate before storage: dates, revisions,
  identifiers, enums, nonnegative measures, and resource bounds.
- Remove semantic drift between local canonicalization and sync
  canonicalization.
- Reconstruct request and turn counts without expanding attacker-controlled
  counts into unbounded synthetic events.
- Enforce header-based CSRF according to the documented contract.
- Make pairing confirmation atomic and retryable across response loss or
  process restart.
- Align rate limiting with the actual reverse-proxy topology.

### Documentation and consent

- Explain local, hosted, public-profile, demo, and Community Rank network
  contracts together in README and security documentation.
- List the exact hosted aggregate fields, retention/deletion controls, and
  device credential behavior.
- Replace the stale “implementation next” hosted ADR with a current accepted
  architecture and an explicit deferred-hardening list.
- Remove unnecessary live-server inventory from architecture documentation.
- Replace undefined “Full” adapter claims with a plain capability/coverage
  explanation.
- Add basic contribution, support, and actionable security-reporting
  documentation.

### Quality gates

- Make MySQL-backed hosted tests mandatory in at least one Linux CI job.
- Exercise the hosted production build in CI and prove it contains no demo
  fixtures.
- Add TypeScript checking as a supported package script and CI gate.
- Fix the existing TypeScript errors rather than excluding tests from the
  compiler.
- Stabilize the inode-replacement test so it proves that an inode actually
  changed before asserting inode-change behavior.
- Ensure release tags cannot publish before core, web, browser, and CLI gates
  pass.
- Stop using release hooks to silently repair module metadata.

### Product presentation

- Clarify the first-screen product sentence: one honest local account of token
  use across coding agents.
- State plainly that the dashboard is Chinese-first rather than leaving
  English readers to infer it from screenshots.
- Replace stale screenshots when they no longer match the 2x5 dashboard.
- Make the theme preview help users judge the dashboard, not an unrelated
  animated keyboard.

## Delete

Deletion is intentionally narrow.

- The in-memory-only pairing credential handoff.
- The partial-body idempotency hash.
- Best-effort ignored database writes in a successful sync response.
- Cookie fallback for operations documented as requiring a CSRF header.
- Duplicate sync request-canonicalization behavior that differs from
  `internal/metric`.
- The unused legacy `evaluation` presentation field once compatibility impact
  is checked; the product exposes one portrait mechanism, not two overlapping
  evaluators.
- The animated keyboard as the full-screen theme preview.
- Release-time `go mod tidy` as an implicit repair step.
- Obsolete operational inventory and “implementation next” wording in the
  hosted ADR.
- The superseded Newsprint implementation specification as active guidance.
  Its useful failure lessons are retained in this decision and source
  provenance; it must not drive another procedural rewrite.

We do **not** delete:

- hosted code as a substitute for fixing correctness;
- Community Rank solely because it is niche—it is already isolated from the
  default product and can remain an advanced self-hosted command;
- the portrait solely because its phrasing is playful—the engine is
  deterministic, local, bounded, and identity-oriented;
- public profiles, publication validation, or Newsprint;
- existing themes that users may already have selected.

## Ignore

These concerns are real but are not worth touching in the v1 stabilization
tranche:

- a framework change for either web surface;
- replacing Vue, Pinia, Vite, GSAP, Go's HTTP stack, or the static profile;
- a generic repository-wide abstraction for every projection type;
- dynamic adapter registration or reflection;
- replacing the index with content-addressed storage;
- a new pricing service or automatic vendor-price scraping;
- automatic public-profile publishing;
- teams, social feeds, billing, global rankings, or raw-event cloud storage;
- adding more adapters without a verified safe ledger;
- a Newsprint PBR pipeline, extra wrinkles, animated lighting, or more noise;
- adding more dashboard or public-profile themes;
- a custom font-distribution project;
- a broad rewrite of `scan`;
- a database-engine migration inside this review branch;
- complete dashboard internationalization inside this review branch.

Internationalization is valuable, but it changes every component, test,
layout, and support promise. For v1 it is a product strategy decision, not a
small polish task. The immediate requirement is honest language labeling and
a documented path, not a rushed translation layer.

## Priority

### P0 — blocks an end-to-end v1 claim

1. Hosted equal-revision comparison covers all persisted measures.
2. Hosted batch idempotency covers the full envelope and commits atomically
   with usage, timezone, source status, and sync metadata.
3. Hosted input validation rejects malformed, negative, and unbounded data.
4. Local and sync canonicalization have one behavior and parity tests.
5. Pair approval survives retries/restarts and cannot create multiple devices
   for one challenge.
6. CSRF requires the documented header proof.
7. Hosted MySQL integration and hosted artifact builds are mandatory CI
   gates.
8. README and security docs disclose hosted sync and distinguish every
   network mode.

### P1 — required for a polished v1 candidate

1. TypeScript checking is green and enforced.
2. The index replacement test is deterministic across supported filesystems.
3. The hosted ADR reflects shipped reality and omits operational archaeology.
4. Release publication is gated and module metadata cannot drift silently.
5. Contribution and security processes are actionable.
6. Hosted readiness reflects database availability and the server shuts down
   normally.
7. Rate limits work behind the documented proxy.
8. Pair/OAuth one-time transitions are race-safe.
9. Current screenshots and first-screen product copy match the actual product.

### P2 — valuable maintainability work after the candidate is stable

1. Give hosted aggregate reconstruction a bounded metric API instead of
   manufacturing one event per count.
2. Reduce `scan` presentation responsibilities behind parity tests.
3. Make unavailable state harder to misuse in new renderers.
4. Mechanically prove dashboard embed/source parity.
5. Remove dual workflow ownership if repository permissions no longer require
   it.
6. Make previous comparison windows calendar-aware across DST.
7. Reduce manual lag across Homebrew, npm metadata, profile publication, and
   hosted deployment.
8. Decide and execute an internationalization strategy.

### P3 — explicitly deferred

- new providers without safe ledgers;
- social or competitive identity features;
- automatic background sync;
- live public profiles;
- more palettes or material effects;
- a new mascot or brand system;
- hosted teams and billing;
- raw cloud events;
- a rewrite of the terminal report or dashboard;
- major or minor version changes.

## Approved implementation tranche

The review branch will implement only changes that are both high impact and
bounded:

1. close the verified hosted sync integrity, validation, CSRF, and pairing
   durability defects without changing the aggregate-only architecture;
2. make the relevant CI, TypeScript, hosted-build, and release checks explicit;
3. fix the nondeterministic index test without changing the documented cache
   model;
4. align README, security, hosted architecture, contribution, and support
   documentation with shipped reality;
5. simplify the theme preview if browser comparison confirms that the
   keyboard adds spectacle rather than useful information;
6. preserve the current Newsprint material and record the decision to stop
   procedural iteration.

Anything outside that tranche requires a later decision.

## Quality gate

The branch is acceptable only when:

- local accounting totals and unavailable semantics are unchanged;
- append-only scan tests remain green;
- hostile sync payloads cannot store invalid counts or cause unbounded
  dashboard reconstruction;
- equal revisions either match completely or conflict;
- retrying an identical full batch is safe;
- pairing can recover approval after a dropped status response;
- all required MySQL tests demonstrably execute in CI;
- local, demo, and hosted web builds remain distinct;
- public-profile browser tests pass in Chromium, Firefox, and WebKit;
- generated assets are verified rather than assumed;
- no visual change is accepted without desktop and mobile browser evidence;
- no release/version bump is made.
