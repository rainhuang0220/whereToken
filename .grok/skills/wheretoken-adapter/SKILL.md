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
- Register the tool in more than `internal/adapter/<id>/`, `adapter.Catalog`, and `scan.Adapters`.

## Must

1. Implement `adapter.Adapter` (`ID`, `Discover`, `Parse`).
2. Add a row to `adapter.Catalog`.
3. Map only fields the source actually has. Set `Quality` and `Derivation`.
4. Fixture under `testdata/adapters/<id>/` (usage fields only).
5. Malformed-line test and secret-leak test.
6. JSONL append → `index.LoadOrParse`. SQLite / cumulative → `index.LoadOrReplay`.
7. Update `docs/data-sources.md` and `docs/token-accounting.md`.

`go test ./internal/adapter/<id>/ ./internal/adapter/ ./internal/scan/` before expanding scope.
