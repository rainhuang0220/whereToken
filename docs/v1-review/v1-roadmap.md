# whereToken v1 roadmap

Date: 2026-09-16  
Product direction: a local-first developer identity built from honest
coding-agent usage

## Implementation outcome (this review branch)

The Gate A correctness items were implemented and verified against real
MySQL/InnoDB (MariaDB 10.11) semantics, including concurrent-request races:

- equal-revision comparison now covers every persisted measure, not only
  `miss`;
- sync idempotency now hashes the full request envelope and applies usage,
  timezone, and source status atomically in one transaction, claiming the
  idempotency key first so a concurrent identical retry cannot double-apply;
- `syncagg.Build` now canonicalizes with `metric.CanonicalEvents`, the same
  function the local report, dashboard, and public profile use, instead of a
  second implementation that assigned duplicate stream rows to the wrong
  local date;
- device pairing is now one locked transaction (`Store.ApprovePair`); the
  CLI-generated `device_secret` is the eventual bearer token itself, so a
  dropped status response or process restart no longer strands the
  credential;
- the hosted CSRF check now requires the `X-CSRF-Token` header with no
  cookie fallback, matching the documented contract (the existing frontend
  already sent the header everywhere, so this closed the gap with no UI
  change);
- `DecodeBatch` now bounds and validates every field (dates, revisions,
  token/count ranges, and quality/derivation/status enums) instead of
  trusting an authenticated bearer's request body.

Gate C privacy documentation was updated: README (English and Chinese) and
SECURITY.md now disclose the hosted sync network path with its exact
uploaded fields, alongside the pre-existing Community Rank and local-only
modes. The hosted ADR's status was corrected from "Proposed" to
"Implemented," and host-specific operational inventory (IP address, SSH key
names, sibling-application ports, stored-password notes) was removed from
version control.

Gate D (visual restraint) was evaluated and no visual change was made. The
design review's only concrete recommendations were taste opinions (replace
the theme-gallery keyboard preview; iterate Newsprint toward more cockling)
rather than defects, and the current Newsprint material already satisfies
this review's standard: real scanned paper, restrained processing, no fake
crease paths, no procedural noise as the primary realism source. Per the
operating principle that a mature maintainer does not maximize changes, both
were left as-is. Real-browser screenshots taken after all backend changes
(`public_profile_newsprint_desktop.png`, `public_profile_newsprint_mobile.png`,
`dashboard_demo_after_hosted_fixes.png`) confirm no regression: the kiln
dashboard, KPI grid, and Newsprint public profile render identically to
before this branch's changes, because no frontend or asset code was touched.

Deferred to a later pass, per the P1/P2 priority list below: hosted
readiness/graceful-shutdown, rate limiting correct for the real proxy
topology, a maintained (non-EOL) database baseline, and the
internationalization strategy decision.

## Product promise

whereToken answers one question:

> Where did my coding-agent tokens go?

It answers with one trustworthy local ledger, one memorable kiln wall, and
one optional public activity profile.

The product is not an invoice, a surveillance service, a generic model
dashboard, or a social network. Its identity comes from making otherwise
fragmented machine records legible without pretending unavailable data is
zero.

## 1. What should users remember?

After opening whereToken, users should remember:

> My agents keep separate receipts. whereToken turns them into one honest
> kiln.

The visual memory is the 53-week kiln wall. The trust memory is that every
number can explain its source and quality. The identity memory is a personal
record of how the developer works with coding agents, not merely a cost
chart.

Three supporting ideas are enough:

1. **Local authority.** The user's ledgers remain the source of truth.
2. **Honest accounting.** Missing data and missing prices stay unavailable.
3. **Publish by choice.** A public profile is a sanitized, manually published
   snapshot, never a hidden live feed.

Everything else is secondary.

## 2. What differentiates whereToken?

### Accounting before visualization

Generic AI dashboards begin with a chart and accept whatever totals an API
returns. whereToken begins with a documented token model:

- tool and provider are different axes;
- cache read, cache create, miss, and output remain distinct;
- reasoning is not charged twice;
- repeated stream rows are canonicalized;
- source quality and derivation are visible;
- unknown price is not free.

### A local developer record

whereToken reads the ledgers coding agents already maintain. The local CLI and
dashboard do not require a whereToken account. This is the default product,
not a degraded offline mode.

### A recognizable instrument

The kiln wall, ember scale, 窑 mascot, and firing vocabulary make usage feel
like a crafted instrument rather than a SaaS template. The identity should
remain restrained:

- one dominant wall;
- strong numerical hierarchy;
- hairline structure;
- no glass cards, glow fields, or ornamental charts;
- no decoration without a job.

### A defensible public profile

The public profile is differentiated by its publication contract:

- strict allowlist;
- explicit data provenance;
- coverage disclosure;
- stable data identity;
- separate renderer revision;
- static, inspectable assets;
- no runtime connection to the owner's machine.

Newsprint strengthens the editorial character through real material and
restrained treatment. It is not an effects showcase.

## 3. Minimal changes for a polished v1

### Gate A — trust

These changes come first because visual polish cannot compensate for an
incorrect synchronized ledger.

- Make hosted retries, revisions, and persistence atomic and complete.
- Validate every hosted aggregate and bound reconstruction work.
- Use one canonical request-merge behavior across local and sync paths.
- Make pairing retryable and race-safe.
- Enforce the documented CSRF contract.
- Require MySQL-backed hosted tests and the hosted production build in CI.
- Make privacy documentation enumerate every network path.

Exit condition: a hosted view cannot silently diverge from an accepted sync
request, and a green build proves the relevant database tests ran.

### Gate B — engineering confidence

- Make standalone TypeScript checking pass and run it in CI.
- Stabilize the filesystem-sensitive index test.
- Gate release publication on the documented core checks.
- Replace release-time dependency repair with drift detection.
- Bring the hosted architecture record up to date.
- Add an actionable contribution and security path.

Exit condition: a new maintainer can make and verify a small change without
guessing which checks or artifacts matter.

### Gate C — product clarity

- Lead README and landing-page copy with the literal product value before the
  kiln metaphor.
- Show local, hosted, public, demo, and Community Rank behavior in one concise
  privacy table.
- State the current Chinese-first interface honestly.
- Refresh screenshots that depict old KPI structures.
- Keep one visual idea dominant on each surface.

Exit condition: a first-time visitor can explain the product, its privacy
model, and the difference between local and hosted operation without reading
the architecture docs.

### Gate D — restraint

- Freeze adapter count for the candidate unless a correctness defect demands
  a change.
- Freeze dashboard and public-profile palettes.
- Replace the theme-gallery keyboard only if the replacement helps users
  judge the actual dashboard.
- Keep the current Newsprint material unchanged unless browser evidence finds
  a concrete defect.
- Remove superseded guidance that encourages another procedural paper pass.

Exit condition: no v1 change exists solely to demonstrate implementation
ability.

## Implementation sequence

### 1. Correctness

1. Add failing focused tests for complete revision equality, full-envelope
   idempotency, invalid aggregates, canonicalization parity, retryable
   pairing, and header-only CSRF.
2. Make the smallest storage and handler changes that satisfy those tests.
3. Verify local accounting output is unchanged.

### 2. Gates

1. Repair TypeScript errors and expose a `typecheck` package script.
2. Make hosted MySQL and hosted build checks mandatory on Linux.
3. Stabilize the index identity test premise.
4. Make tag publication depend on tests.

### 3. Public truth

1. Update README and security documentation.
2. Replace the stale hosted ADR with current architecture and deferred work.
3. Add contribution and support guidance.
4. Refresh only demonstrably stale generated media.

### 4. Visual decision

1. Compare current theme-gallery and Newsprint surfaces in a real browser.
2. Change only the theme preview if the actual dashboard is a clearer
   decision aid than the keyboard.
3. Capture desktop and mobile before/after evidence.
4. Regenerate and validate every affected artifact.

## Candidate success criteria

A v1 candidate is credible when:

- a clean checkout passes all documented gates;
- local and hosted totals reconcile for the same aggregate input;
- malformed hosted input fails before storage;
- repeated full batches are safe and changed repeats conflict;
- no successful response hides a failed metadata write;
- a lost pairing response can be retried;
- the README's privacy claims match the code;
- public profiles remain static, sanitized, and provenance-gated;
- the dashboard still looks unmistakably like whereToken;
- Newsprint reads as real paper without announcing an effect;
- screenshots match the shipped interface;
- no source, cost, or profile state invents a zero.

## 4. What should explicitly not be built?

Not for v1:

- more coding-agent adapters without a verified safe ledger;
- automatic vendor price scraping;
- raw event, prompt, transcript, path, request, or session upload;
- a public Community Rank deployment;
- global or worldwide rank claims;
- teams, organizations, billing, or subscriptions;
- a social feed, following system, badges, or share counters;
- automatic public-profile commits;
- live public profiles connected to a local machine;
- background sync daemons or `sync --watch`;
- an LLM-generated portrait;
- runtime WebGL, paper physics, procedural wrinkles, or additional Newsprint
  effects;
- more themes or a new design system;
- a framework migration;
- a scanner, metric, report, or public-profile rewrite;
- a generic plugin architecture;
- content-addressed indexing;
- a database migration undertaken only for novelty;
- a rushed full-interface translation.

## Post-v1 recommendations

Only after the candidate is stable:

1. decide whether bilingual UI is a product requirement and design it as a
   complete content/layout system;
2. introduce a bounded aggregate API for hosted reconstruction;
3. simplify `scan` responsibilities behind parity tests;
4. move hosted storage to a maintained database baseline with explicit
   migrations;
5. automate release-channel synchronization;
6. evaluate public-profile intensity against a wider anonymized fixture set;
7. revisit advanced self-hosted Community Rank only if real operators use it.

## Final test

Before calling the release v1, ask:

- Can a stranger understand the product before understanding the metaphor?
- Can a senior engineer trace a number to its source and quality?
- Can a user distinguish local analysis from every optional upload?
- Does the public profile make the developer look interesting, not the
  dashboard look busy?
- Does every visual element help recognition, reading, state, or action?
- Is every remaining complexity justified by a shipped contract?

If any answer is no, the candidate is not finished.
