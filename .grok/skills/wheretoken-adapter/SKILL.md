---
name: wheretoken-adapter
description: Add or fix a whereToken usage adapter. Use when adding a coding-agent collector, mapping token fields, or changing Discover/Parse so missing usage is not shown as zero.
---

# Add or fix a whereToken adapter

Follow [`docs/adding-an-adapter.md`](../../../docs/adding-an-adapter.md) and [`docs/token-accounting.md`](../../../docs/token-accounting.md).

## Do not

- Treat a config/home directory as usage.
- Emit `0` when the ledger is missing or encrypted.
- Read credentials, prompts, or mixed auth+body SQLite.
- Register the tool in more than `internal/adapter/<id>/`, `adapter.Catalog`, and `scan.Adapters` (contract tests use `scan.Adapters`).

## Must

1. Implement `adapter.Adapter` (`ID`, `Discover`, `Parse`).
2. Add a row to `adapter.Catalog` with `Caps` (discovery ≠ usage).
3. Map only fields the source actually has. Set `Quality` and `Derivation`.
4. Fixture under `testdata/adapters/<id>/` (usage fields only).
5. Malformed-line test and secret-leak test.
6. JSONL append → `index.LoadOrParse`. SQLite / JSON / cumulative → `index.LoadOrReplay`.
7. Update `docs/data-sources.md`, `docs/token-accounting.md`, `docs/provider-matrix.md`.
8. Pricing belongs in `internal/price` only with a public list URL. Context-tiered / missing component = unavailable, never `$0`.
9. Incremental / window / duplicate RequestID tests when the source can produce those cases.

`go test ./internal/adapter/<id>/ ./internal/adapter/ ./internal/scan/` before expanding scope.
