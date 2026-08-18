# Adding an adapter

A new coding agent is accepted when it has a reliable usage source, a mapping
into the normalized token model, fixtures, and tests. Guessing a number is
worse than leaving the agent unsupported.

## Checklist

1. Add an adapter package under `internal/adapter/<id>/` that implements `adapter.Adapter`.
2. Implement `ID()` (stable, lowercase, non-empty).
3. Implement `Discover(home)` — look only under `adapter.Home` paths. Missing directories are silent, not errors.
4. Implement `Parse` — read the ledger, emit `event.UsageEvent` / `event.TurnEvent`. Skip credential files. Never put JWTs or API keys on events or in errors.
5. Define the token mapping in [`docs/token-accounting.md`](token-accounting.md) and [`docs/data-sources.md`](data-sources.md). Mark each field `raw`, `provider_api`, or `derived`.
6. Set `Quality` and `Derivation` on every usage event (`event.Quality*` / `event.Derive*`).
7. Add a desensitized fixture under `testdata/adapters/<id>/` (usage fields only; no prompts, no secrets).
8. Add a malformed-input test: bad JSON / truncated rows must not panic and must not drop later good rows.
9. Add a secret-leak test: a fake `sk-…` / JWT in a neighboring file must not appear on events or in errors.
10. Add regression tests for the mapping and for any dedup / delta / stream rule.
11. Register the adapter in `scan.Adapters` and `metric.KnownSourceIDs`.
12. Update `docs/data-sources.md` and the supported-agents table in the README.

CI is `go test ./...` plus the web suite. A new adapter should not require editing eight unrelated packages by hand.

## Contract

`internal/adapter/contract_test.go` already checks every registered adapter for:

- non-empty `ID()`
- `Discover` on an empty home does not panic
- `Parse` of junk JSONL does not panic, does not emit negative tokens, and does not leak a planted secret

Add source-specific cases next to the adapter (`*_test.go`) for duplicates, streaming placeholders, and missing fields.

## Incremental scan

JSONL that is append-only should call `index.LoadOrParse` so a later scan reads
only new bytes. Parsers that need running state (Codex cumulative totals) or
that read SQLite should call `index.LoadOrReplay` (unchanged file → replay;
any change → full parse).

The index is a cache. `wheretoken rebuild` deletes it.

## Do not

- Treat missing usage as zero.
- Estimate USD.
- Read `auth.json`, Keychain, Cookies, or credential tables.
- Use a per-line uuid as `RequestID`.
- Add the agent because the directory exists if the files have no token fields.
