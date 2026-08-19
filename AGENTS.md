# whereToken agent notes

Local-first token usage accounting. Read these before changing parsers or totals.

## Sources of truth

- Product token math: [`docs/token-accounting.md`](docs/token-accounting.md)
- Adapter contract: [`docs/adding-an-adapter.md`](docs/adding-an-adapter.md), [`docs/data-sources.md`](docs/data-sources.md)
- Register a tool in `internal/adapter/<id>/`, `adapter.Catalog`, and `scan.Adapters` only. Completions rewrite `--tool` from Catalog.
- Global research table: [`docs/provider-matrix.md`](docs/provider-matrix.md)
- Cost: [`docs/cost.md`](docs/cost.md)
- Community Rank: [`docs/community.md`](docs/community.md)

## Hard rules

- Missing usage is unavailable, never `0.00 M` / `$0` / `#0`.
- Do not invent adapters from a config directory. Finding `~/.foo` is not finding usage.
- Do not read `auth.json`, Keychain, Cookies, credential tables, or mixed auth+transcript SQLite (OpenClaw `agent/`, Trae SQLCipher).
- Do not put prompts, JWTs, or paths on events or in errors.
- Do not estimate USD. Price only from the public list card. Unknown / missing component rate = unavailable.
- Do not add reasoning into Total. Grok / MiniMax reasoning is not a second output charge.
- Do not open a public Community Rank URL. No HMAC theater; UUID is a bearer id for a self-hosted board.
- Do not bump to v0.5.0 unless asked. Patch/minor on 0.4.x only.

## Scan invariant

If source files only append, a later successful scan must not drop tokens.
`/reset` archives (`*.jsonl.reset.*`, `*.jsonl.deleted.*`) still count.
Incremental parse errors must keep the cached events for that file.

## Verify

```bash
go test ./...
go vet ./...
```
