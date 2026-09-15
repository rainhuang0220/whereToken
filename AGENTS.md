# whereToken agent notes

Local-first token usage accounting. Read these before changing parsers or totals.

## Sources of truth

- Product token math: [`docs/token-accounting.md`](docs/token-accounting.md)
- Adapter contract: [`docs/adding-an-adapter.md`](docs/adding-an-adapter.md), [`docs/data-sources.md`](docs/data-sources.md)
- Register a tool in `internal/adapter/<id>/`, `adapter.Catalog`, and `scan.Adapters` only. Completions rewrite `--tool` from Catalog.
- Global research table: [`docs/provider-matrix.md`](docs/provider-matrix.md)
- Cost: [`docs/cost.md`](docs/cost.md); the estimator and `wheretoken pricing` read one table — rates in `internal/price/table.go`, official sources + verification dates in `internal/price/catalog.go`
- Model-level estimate: `by_model` (`/api/summary`, dashboard 估价 modal, `wheretoken pricing --usage`) prices through `price.Resolve`; model ids normalize (version-first → family-first) for matching only, raw ids stay on events.
- User portrait: `internal/profile` — deterministic bucketed trait vector + Chinese phrase bank, seeded by the local anonymous install id only (never username/HOME/hostname/IP/credentials). No LLM, no wall-clock randomness, no rank claims. Web reads it from the summary JSON; the CLI report renders the same engine via `report.Snapshot.Portrait` (bottom-right KPI). Rank never appears in the default report — community rank lives in `wheretoken community` and the `--json` community block only.
- Community Rank: [`docs/community.md`](docs/community.md)

## Hard rules

- Missing usage is unavailable, never `0.00 M` / `$0` / `#0`.
- Do not invent adapters from a config directory. Finding `~/.foo` is not finding usage.
- Do not read `auth.json`, Keychain, Cookies, credential tables, or mixed auth+transcript SQLite (OpenClaw `agent/`, Trae SQLCipher).
- Do not put prompts, JWTs, or paths on events or in errors.
- Do not estimate USD. Price only from the public list card. Unknown / missing component rate = unavailable.
- Do not add reasoning into Total. Grok / MiniMax reasoning is not a second output charge.
- Do not open a public Community Rank URL. No HMAC theater; UUID is a bearer id for a self-hosted board.
- Do not bump the minor or major version unless asked. Patch on 0.7.x only.

## Scan invariant

If source files only append, a later successful scan must not drop tokens.
`/reset` archives (`*.jsonl.reset.*`, `*.jsonl.deleted.*`) still count.
Incremental parse errors must keep the cached events for that file.

## Installer invariants

- Installation success is user-observable command availability, not merely copying a binary. Windows tests must prove install → resolve `wheretoken` → `--version` → run in the same shell process.
- Do not hide installer defects with README advice to reopen the terminal. The normal path is one install command, then `wheretoken` in that same terminal.
- `cmd.exe` installers must preserve caller state deliberately across `setlocal`; use `call` when the parent batch must continue after the installer returns.
- Treat existing PATH values as opaque data. Do not parse them with delayed expansion or unquoted shell metacharacters; preserve spaces, literal `!`, `&`, `%VAR%`, and existing entries exactly.
- Installer regression tests must exercise the actual shell lifecycle. Separate GitHub Actions `run:` steps are not proof of same-shell PATH behavior.
- Keep normal installs user-scoped and dependency-free when release binaries exist; a download failure must fail clearly rather than silently falling back to a toolchain such as Go.
- Distinguish installer-script rollout from packaged release rollout: changes under `main/scripts/install.*` are live as soon as main changes, while the latest binary version remains whatever the most recent GitHub Release published.

## Verify

```bash
go test ./...
go vet ./...
```
