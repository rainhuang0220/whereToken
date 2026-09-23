# ADR: whereToken Hosted Web App v0.7.0

**Status:** Implemented in `v0.7.0` (`internal/hosted`, `cmd/wheretoken-hosted`, `wheretoken login`/`sync`/`logout`). Sections below describe the accepted architecture as designed; see [`docs/v1-review/architecture-review.md`](../v1-review/architecture-review.md) for a post-implementation correctness review and the hardening work it drove.  
**Original planning date:** 2026-09-04  
**Stable base at the time:** `v0.6.4`.

This document records the architecture for turning `https://wheretoken.plainlist.space` into a login-gated app that shows **the signed-in user's real whereToken metrics**. Every choice below exists because of whereToken's local scanner, Cursor account-global usage, price/portrait engines, and privacy boundary. It is not a port of any other product.

### Status update (post-implementation)

The design below shipped substantially as planned: GitHub OAuth + PKCE, device pairing, daily-by-model sync, `source_scope` REPLACE/SUM semantics, and hosted dashboard reconstruction through the shared `internal/metric` engine. The v1 maturity review found and fixed several correctness gaps against this design — equal-revision comparison covering only one column, idempotency keyed on a partial request body, non-atomic sync persistence, an in-memory-only pairing credential, and a CSRF cookie fallback that didn't match the header-only contract described below. Those fixes live in `internal/hosted` and `internal/syncagg`; this document is not re-litigated line by line to match every implementation detail, so treat mismatches between prose here and current code as documentation drift to fix, not as evidence the running service does something different on purpose.

Deferred, not yet done: mandatory (not best-effort) rate limiting correct for the actual reverse-proxy topology, a maintained (non-EOL) database baseline with a real migration ledger, process-level graceful shutdown, and a `/api/health` check that verifies database reachability rather than only that the process is running.

---

## Discovery (at original design time)

This table is the starting inventory this ADR was written against. It intentionally omits server hostnames/IPs, SSH key names, sibling-application inventory, and other host-specific operational details that do not belong in a public repository; that information lives in the maintainer's private deployment notes, not here.

| Fact | Value |
| --- | --- |
| CLI commands at the time | report (default), serve, scan, sources, doctor, rebuild, update, uninstall, completion, community, pricing. **No login/sync yet.** |
| Local dashboard | `wheretoken serve` → `127.0.0.1:8787–8797` → `GET /api/summary` + `POST /api/scan`. Non-localhost bind refused. |
| Go module deps at the time | `go-isatty`, `modernc.org/sqlite`. **No MySQL, OAuth, session, or Keychain library yet.** |
| Project site | `https://rainhuang0220.github.io/whereToken/` (GitHub Pages). Demo at `/whereToken/demo/`. |
| Hostname before this work | Served a `VITE_DEMO=1` build behind the intended production hostname — a public synthetic demo standing in for the product. This ADR's v0.7.0 work replaces that root with the real login-gated app. |

---

## Product seam

```
Cursor / Claude / Codex / Grok / …
                │
                ▼
        Local whereToken
     scan / normalize / aggregate / price
                │
                │ authenticated sync of daily×model metrics
                ▼
       Hosted whereToken API
     identity + device + upsert + re-price + portrait
                │
                ▼
https://wheretoken.plainlist.space
     login / pair / real dashboard / devices / privacy
```

| Surface | Authority |
| --- | --- |
| Local scanner (`internal/adapter`, `internal/scan`) | What happened on this machine |
| Hosted backend (`cmd/wheretoken-hosted`) | Who the user is, which devices may write, how rows merge |
| Hosted web (`web/` + `VITE_HOSTED=1`) | Visualization of **already stored** metrics |

The hosted process never opens `~/.cursor`, JSONL, SQLite, Keychain, or `auth.json`. The hosted page never `fetch`es `http://127.0.0.1`. A localhost bridge is out of v0.7.0.

Community Rank (`internal/community`, `wheretoken community`) stays an optional self-hosted board. Hosted Web is not a rank product.

---

## 1. Local scanner vs cloud

**Decision:** two processes, two muxes, one domain model.

- `wheretoken serve` remains the **localhost kiln**. `internal/httpapi` keeps `Listen` refusing non-loopback binds, Host/Origin checks, `/api/summary`, `/api/scan`, `/api/community`. Tests that pin DNS-rebinding (`TestScanRejectsNonLocalHost`, …) stay green.
- Hosted is a **new binary** `cmd/wheretoken-hosted` + package `internal/hosted`. It does not register `scan.Adapters`. It does not import the local mux.
- Shared packages (read-only reuse): `internal/event`, `internal/metric`, `internal/price`, `internal/profile`, `internal/insight`, `internal/scan` **JSON builders** (`MarshalSummary` / view helpers), `internal/report.Redact`.

Why this fits whereToken: the local HTTP API is a single-user file reader. Opening it on a public port would be remote filesystem access. Sharing `metric`/`price`/`profile` keeps 总用量 / 估价 / 用户画像 identical to the CLI without a second implementation.

CLI sync calls the same scan path the report already uses:

```
App.doScan → scan.Run / RunWithProgress → mergeByRequest → daily×model rows → PUT /api/v1/sync/batch
```

The index invariant is unchanged: later successful scans must not drop tokens; `/reset` archives still count; parse errors keep cached events. Sync uploads **after** that local merge.

---

## 2. Web login

**Decision:** dedicated whereToken GitHub OAuth App. GitHub is identity only.

Flow:

```
GET  /api/v1/auth/github
  → 302 GitHub authorize (state + PKCE S256, no repo scopes)
GET  /api/v1/auth/github/callback?code&state
  → verify state
  → exchange code
  → GET https://api.github.com/user  (id, login, avatar_url)
  → discard GitHub access token
  → upsert users row on github_id
  → mint whereToken web session, rotate cookie
  → 302 /app
```

| Item | Value |
| --- | --- |
| OAuth App name | `whereToken` |
| Homepage URL | `https://wheretoken.plainlist.space` |
| Authorization callback URL | `https://wheretoken.plainlist.space/api/v1/auth/github/callback` |
| Scopes | none (public profile: immutable user id, login, avatar). **Do not** request `repo`, `workflow`, `user:email`. |
| Client secret | server env `WHERETOKEN_GITHUB_CLIENT_SECRET` only, file mode `0600`. Never git, never the CLI binary. |
| Client id | `WHERETOKEN_GITHUB_CLIENT_ID` |

Email is not a product requirement for v0.7.0 (no billing, no transactional mail). Skip `user:email`.

Web session:

- Cookie name `wt_session`
- Value: opaque 32-byte random, URL-safe. Server stores **SHA-256 only**
- Flags: `HttpOnly; Secure; SameSite=Lax; Path=/`
- TTL: 30 days. Logout deletes the row. Login rotates (old hash dropped)
- CSRF: mutating cookie-auth routes require header `X-CSRF-Token` matching a session-bound token issued by `GET /api/v1/session`

Why not PlainList JWT: that token is another product's session, scoped to another database, with no device/sync claims. Why not keep the GitHub access token: whereToken never calls GitHub again after identity.

Unauthenticated `GET /` → `/login`. Authenticated `GET /` → `/app`. `401` on cookie APIs → `/login`.

---

## 3. CLI pairing

**Decision:** whereToken device pairing. The CLI never contains an OAuth client secret and never stores a GitHub token.

```
wheretoken login
  POST /api/v1/pair/start
    body: { client_version, os, arch, label? }
    → { display_code, device_secret, expires_at, verification_url, poll_url }
  print verification_url, open browser
  poll POST /api/v1/pair/status  { display_code, device_secret }
    pending | denied | expired | { device_token, user_login, device_id }
  store device_token

wheretoken logout
  POST /api/v1/devices/self/revoke  (bearer) then delete local secret
```

Browser page `/pair/:code`:

1. If no web session → GitHub login, return to `/pair/:code`
2. Show device label, `os/arch`, client version. **Confirm / Reject**
3. Confirm requires the logged-in user. The display code alone cannot finish the bind

`display_code` is Crockford Base32, 8 characters shown as `ABCD-EFGH`. It is an index, not a bearer. `device_secret` is 32 bytes from `crypto/rand`, returned **once** at start. Status poll must present both. Guessing the short code without the secret fails. Replay after success fails (challenge single-use). TTL 10 minutes.

Rate limits (per IP unless noted): `pair/start` 10/min; `pair/status` 30/min per challenge; confirm 20/min per user. Brute-force on the short code is additionally useless without `device_secret`.

Device metadata uploaded at start: `os`, `arch`, `client_version`, optional user-typed `label`. Default label is `macOS arm64` / `Windows amd64` style — **not hostname**. Hostname is not sent.

`wheretoken sync` is the v1 closed loop. `sync --watch` and OS agents wait until explicit sync is green.

Commands added in `internal/cli/flags.go` `applyCommandWord` + `App.Run` + help + four completion scripts. They are siblings of `pricing`, not children of `community`.

---

## 4. Device token lifecycle

| Stage | Rule |
| --- | --- |
| Issue | Only when a pairing challenge is **confirmed**. Raw token returned once. |
| Format | `wtd_1.` + 32 random bytes, base64url. Prefix makes accidental log grep easy to redact. |
| Server storage | `devices.token_hash` = SHA-256(raw). Never the raw token. `token_version` for rotation. |
| CLI storage | macOS Keychain (`whereToken` / `device-token`); Linux secret-service / Windows credential manager; **0600** file `~/.config/wheretoken/device-token` (Windows `%APPDATA%\whereToken\device-token`) only if the OS store is unavailable. |
| Use | `Authorization: Bearer` on `/api/v1/sync/*`, heartbeat, self-revoke. **Not** cookies. CSRF does not apply. |
| Last seen | Updated on successful authenticated call. |
| Revoke | Settings → Devices → Revoke sets `revoked_at`. Subsequent bearer auth fails immediately. Logout is self-revoke + local delete. |
| Rotation | Optional later. v1 revoke+re-pair is enough. |

Device APIs are scoped to the device's user. User A cannot list, revoke, or read User B's devices (IDOR tests).

---

## 5. Sync data contract

**schema_version 1.** Daily × model aggregates. No event stream.

```jsonc
{
  "schema_version": 1,
  "idempotency_key": "<uuid per HTTP attempt>",
  "device_id": "<server-issued public id>",
  "generated_at": "2026-09-04T12:00:00+08:00",
  "timezone": "Asia/Shanghai",
  "price_catalog_version": "2026-08-19",
  "client_version": "0.7.0",
  "sources": [
    {
      "tool": "cursor",
      "detected": true,
      "status": "ok",
      "quality": "authoritative",
      "source_scope": "account_global",
      "source_key_hash": "<hex>"
    }
  ],
  "daily_model_usage": [
    {
      "date": "2026-09-03",
      "tool": "cursor",
      "source_scope": "account_global",
      "source_key_hash": "<hex>",
      "vendor": "anthropic",
      "model": "claude-4.6-opus-high-thinking",
      "miss": 0,
      "cache_read": 0,
      "cache_create": 0,
      "output": 0,
      "requests": 0,
      "user_turns": 0,
      "quality": "authoritative",
      "derivation": "provider_api",
      "revision": 1
    }
  ]
}
```

Field names follow `event.UsageEvent` (`Miss`/`CacheRead`/`CacheCreate`/`Output`), not a new vocabulary. `requests` is counted **after** `SkipRequest` (Cursor API token rows do not inflate request counts). `user_turns` comes from `TurnEvent` counts, not from request rows.

**Never in the payload** (contract test, same spirit as `community.ForbiddenUploadKeys`):

`prompt`, `content`, `transcript`, `path`, `filename`, `workspace`, `project`, `source_root`, `session`, `session_id`, `request_id`, `jwt`, `cookie`, `authorization`, `api_key`, `credential`, `events`, `usage_events`, `sqlite`, `hostname`, `email`, `github` (token), raw DB blobs.

`client_estimate` may be sent as an audit hint. The server ignores it when computing 估价.

Limits: `MaxBytesReader` 2 MiB; max 20k `daily_model_usage` rows; max 32 sources; model string ≤ 128 runes; unknown `schema_version` → 400.

Idempotency:

- Header `Idempotency-Key` (or body `idempotency_key`): identical key + identical body → same 200 without applying twice
- Per-row `revision` (monotonic uint). Server rejects a row with `revision` < stored (stale). Equal revision + equal measures → no-op. Equal revision + different measures → 409
- Out-of-order destructive updates are refused

Incremental: client may send only rows whose local revision advanced. First sync sends the available history (Cursor API is ~53 weeks; that is enough for the kiln wall).

---

## 6. `source_scope` and Cursor dedupe

whereToken is not a generic request logger. Cursor's DashboardService usage is **the same ~53-week account window on every machine**. Summing Mac + Windows doubles 总用量.

**Decision:** adapters, not the frontend, mark scope.

| Scope | Who | Hosted merge |
| --- | --- | --- |
| `account_global` | Cursor **API token rows** (`Derivation=provider_api`, `SkipRequest=true`) | **REPLACE** on `(user_id, tool, source_key_hash, date, vendor, model)` |
| `device_local` | Every local ledger (Claude, Codex, Grok, …), Trae session API, Cursor **local** request/turn rows (and local tokens only when the API supplied none) | **SUM** across devices at read on `(user_id, device_id, tool, source_key_hash, date, vendor, model)` |

Cursor one sync may emit **both** families. Local bubble tokens are already zeroed when the API has totals (`stripLocalTokens`). Hosted must not invent a third mix.

`source_key_hash` is computed **on the device**, never from a path, and never as `SHA256(email)` / `SHA256(username)` (those preimages are enumerable).

The HMAC key is a **per-whereToken-user** 32-byte secret (`users.source_hmac_key`), issued at pairing and stored with the device credential. It is the same on every device of that account, so two laptops hashing the same Cursor `sub` REPLACE instead of SUM. A per-install pepper would break that.

```
hex(HMAC-SHA256(user_source_hmac_key, "wheretoken/v1/" || tool || "/" || scope || "/" || stable_local_id))
```

For Cursor `account_global`, `stable_local_id` is the JWT `sub` (or the product user id from the usage API), used only as HMAC message and **never written to events, logs, or the payload**. For `device_local`, `stable_local_id` is the server-issued `device_id`. Trae stays `device_local`: its API is per session discovered on that machine; REPLACE-by-account would drop the other laptop's sessions.

Timezone: `date` is the same local calendar date `metric.BuildCalendar` already uses (`time.Local` on the scanning device). Multi-TZ account_global collision is a v1 documented limitation; we do not invent a second calendar.

---

## 7. DB schema

Engine: the box already runs MySQL 5.7. **New database `wheretoken`**, new user `wheretoken`@`127.0.0.1`, least privilege (`SELECT, INSERT, UPDATE, DELETE` on `wheretoken.*` only). Not `plainlist`, not `fire_db`.

```sql
-- users
id              BIGINT PK
public_id       CHAR(36) UNIQUE NOT NULL   -- UUID, appears in APIs
github_id       BIGINT UNIQUE NOT NULL
github_login    VARCHAR(255) NOT NULL
avatar_url      VARCHAR(1024) NOT NULL DEFAULT ''
profile_seed    CHAR(36) NOT NULL          -- UUID, portrait only
source_hmac_key BINARY(32) NOT NULL        -- HMAC key for source_key_hash; never leave the pairing response in logs
created_at, updated_at
deleted_at      DATETIME NULL

-- web_sessions
id              BIGINT PK
user_id         BIGINT NOT NULL
token_hash      BINARY(32) UNIQUE NOT NULL
csrf_hash       BINARY(32) NOT NULL
expires_at      DATETIME NOT NULL
rotated_from    BIGINT NULL
created_at, last_seen_at

-- oauth_states  (short TTL; PKCE verifier stored hashed)
state_hash      BINARY(32) PK
code_verifier_hash BINARY(32) NOT NULL
expires_at      DATETIME NOT NULL
created_at

-- devices
id              BIGINT PK
user_id         BIGINT NOT NULL
public_id       CHAR(36) UNIQUE NOT NULL
label           VARCHAR(128) NOT NULL
os              VARCHAR(32) NOT NULL
arch            VARCHAR(32) NOT NULL
client_version  VARCHAR(32) NOT NULL
token_hash      BINARY(32) UNIQUE NOT NULL
token_version   INT NOT NULL DEFAULT 1
created_at, last_seen_at, last_sync_at
revoked_at      DATETIME NULL

-- pairing_challenges
id              BIGINT PK
display_code    CHAR(8) UNIQUE NOT NULL
secret_hash     BINARY(32) NOT NULL
user_id         BIGINT NULL          -- set at confirm
device_meta     JSON NOT NULL        -- os/arch/version/label
expires_at      DATETIME NOT NULL
consumed_at     DATETIME NULL
denied_at       DATETIME NULL
created_ip_hash BINARY(32) NULL      -- hash, not raw IP as user data

-- usage_daily_model
user_id         BIGINT NOT NULL
device_id       BIGINT NOT NULL      -- 0 = account_global canonical row (MySQL 5.7 UNIQUE treats NULL as distinct)
source_scope    ENUM('device_local','account_global') NOT NULL
tool            VARCHAR(32) NOT NULL
source_key_hash CHAR(64) NOT NULL
date            DATE NOT NULL
vendor          VARCHAR(32) NOT NULL
model           VARCHAR(128) NOT NULL  -- raw id
miss, cache_read, cache_create, output, requests, user_turns  BIGINT NOT NULL
quality         VARCHAR(32) NOT NULL
derivation      VARCHAR(64) NOT NULL
revision        BIGINT NOT NULL
writer_device_id BIGINT NULL         -- which device last wrote an account_global row
updated_at      DATETIME NOT NULL

-- unique:
-- account_global: (user_id, tool, source_key_hash, date, vendor, model)
-- device_local:   (user_id, device_id, tool, source_key_hash, date, vendor, model)

-- source_states
user_id, device_id, tool, source_scope, source_key_hash
status, quality, detected
updated_at
-- unique (user_id, device_id, tool, source_scope, source_key_hash)

-- sync_revisions
user_id, device_id
idempotency_key  CHAR(36)
body_hash        BINARY(32)
schema_version   INT
created_at
UNIQUE (device_id, idempotency_key)
```

Why daily×model, not events: the dashboard needs Today / 7d / 30d / All / custom range, model breakdown, estimate, portrait, kiln wall. Those all reconstruct from per-day token buckets. Events would upload session ids and workspaces we have just forbidden.

Hosted dashboard rebuild (no second math):

```
rows in window
  → one synthetic UsageEvent per row (timestamp = that local date at noon in the batch timezone)
  → optional TurnEvent stubs from user_turns
  → metric.AggregateAt
  → profile.Evaluate(sum, users.profile_seed)
  → insight.Evaluate / insight.Lines (optional parity)
  → same JSON shape as scan.MarshalSummary
  → envelope: devices, last_sync_at, source_states, never_synced
```

Pricing: `AggregateAt` already calls `price.Event` / `price.Resolve`. Hosted 估价 is the same card (`price.CardVersion` currently `"2026-08-19"`). When the card changes in a later CLI release, the server binary that contains `internal/price` re-prices stored buckets. Client USD is not truth.

Drill `workspaces` / `sessions` are **omitted** on Hosted (they need workspace paths / session ids). `drill.models` may be filled from raw `model`. Local kiln keeps full drill.

---

## 8. Local / Hosted Dashboard code sharing

**Decision:** one Vue app, three **build-time** modes. Do not fork `web/`.

| Mode | Flag | Data |
| --- | --- | --- |
| Local embed | default | `GET /api/summary` on loopback (`wheretoken serve`) |
| Pages demo | `VITE_DEMO=1`, base `/whereToken/demo/` | `public/sample/*.json` only |
| Hosted | `VITE_HOSTED=1`, base `/` | `GET /api/v1/dashboard/summary` with cookies. **Never** sample JSON |

Seam: `web/src/api.ts` (`fetchSummary` / `rescan`) plus `web/src/demo.ts`. Add `isHosted()`. The Pinia store and `KpiRow` / `EstimateModal` / kiln wall stay.

`rescan` on Hosted does **not** POST local `/api/scan`. It refetches the cloud summary. Sync remains `wheretoken sync` on the device.

New routes (vue-router, `createWebHistory`):

| Path | Page |
| --- | --- |
| `/login` | GitHub button |
| `/pair/:code` | Confirm device |
| `/app` | Existing Home kiln + hosted status strip |
| `/settings/devices` | list / revoke |
| `/settings/privacy` | what is / is not uploaded; delete data / account |

Keep `/themes/:id?`. Do not add a marketing site; GitHub Pages remains the brochure.

Hosted never-sample guarantees:

1. Hosted build does not set `VITE_DEMO`
2. Hosted dist **excludes** `sample/` (vite plugin or explicit omit)
3. `fetchSummary` in hosted mode has no sample branch
4. Tests: hosted fetch never requests `sample/*.json`; empty store → onboarding copy, not fixture totals

2×5 KPI unchanged:

```
总用量 | 命中率 | 最长连烧 | 当日用量 | 估价
当前连烧 | 请求 | 用户回合 | 单日最高 | 用户画像
```

No Rank cell. Portrait is server JSON (`profile.Evaluate`). 估价 modal is server `by_model`.

Status strip (Hosted only):

```
MacBook Pro · 已连接
最后同步：18 秒前
Cursor 正常 · Grok 正常 · Claude 正常 · Trae 登录失效
```

Offline device: keep last rows, banner “设备离线，正在展示最后一次同步数据”, show `last_sync_at`. Never wipe the grid.

Never synced: onboarding (install CLI → `wheretoken login` → `wheretoken sync`). **No demo numbers.**

---

## 9. Pricing semantics

One table: `internal/price/table.go` + provenance `internal/price/catalog.go`. Hosted binary links the same package.

| Rule | Enforcement |
| --- | --- |
| API-equivalent list cost, not a subscription bill | Existing copy on the 估价 cell + honesty note |
| Unknown / missing component rate → unavailable, never `$0` | `price.Status` / `FormatCostUSD` |
| Listed free (Z.ai `CreateFree`) → `$0` 限免 | existing |
| Reasoning not in Total, not a second output charge | daily rows do not include `Reasoning` |
| version-first model ids normalize for matching only | client uploads **raw** `model`; server `price.Normalize` / `Resolve` |
| `price_catalog_version` on each batch | `price.CardVersion` (`"2026-08-19"` today) |

`wheretoken pricing` and Hosted 估价 cannot drift unless someone copies the table into JS or SQL. That copy is forbidden.

---

## 10. Portrait semantics

`profile.Evaluate(metric.Summary, seed)` stays the only engine.

| Surface | Seed |
| --- | --- |
| Standalone CLI / `wheretoken serve` | `profile.IdentityFor(home)` — community UUID or `install-id`. **Unchanged.** |
| Hosted dashboard | `users.profile_seed` minted at account creation (UUID) |

Logged-in CLI does **not** silently overwrite the local install-id. Optional later: `wheretoken login` may print that Web portrait uses the account seed. v0.7.0 does not re-seed local files.

`total == 0` → `—`; insufficient tokens → `数据不足`; no LLM; no rank claims.

---

## 11. Privacy

First `wheretoken sync` and `/settings/privacy` show the same list, not a ToS dump:

**Uploaded:** calendar date, tool id, vendor, raw model id, miss / cache read / cache create / output, requests, user turns, source status enum, opaque `source_key_hash`, device public id, timezone, price catalog version, client version.

**Not uploaded:** prompts, conversation text, source code, absolute paths, HOME, hostname, API keys, OAuth/GitHub tokens, raw SQLite/JSONL, session/request ids, workspace paths, community UUID as account id.

Deletion (real rows, not UI-only):

| Action | Effect |
| --- | --- |
| Revoke device | `revoked_at`; bearer fails; `device_local` rows of that device remain until data delete |
| Delete synced data | DELETE `usage_daily_model`, `source_states`, `sync_revisions` for the user |
| Delete account | all of the above + sessions + devices + pairing rows + `users` row. GitHub identity can recreate an **empty** account later |

---

## 12. Deployment

```
nginx
  server_name wheretoken.plainlist.space;
  listen 80  → ACME + 301 HTTPS
  listen 443 → SPA + API
    location /api/  → http://127.0.0.1:3400
    location /      → <hosted release directory>/
                      try_files $uri $uri/ /index.html
                      (Hosted dist, not VITE_DEMO)

wheretoken-hosted
  bind 127.0.0.1:3400
  process supervisor: whereToken's maintainer-managed choice
  env file: 0600, outside the web root
```

Do not proxy the whole site to Go if nginx can serve hashed assets; the API stays loopback. Do not bind `0.0.0.0:3400`. Host-specific paths, process-supervisor configuration, and any sibling applications sharing the deployment host are documented in the maintainer's private operational notes, not in this repository.

Env (names, not values):

```
WHERETOKEN_GITHUB_CLIENT_ID
WHERETOKEN_GITHUB_CLIENT_SECRET
WHERETOKEN_MYSQL_DSN          # user wheretoken, database wheretoken, 127.0.0.1
WHERETOKEN_PUBLIC_URL         # https://wheretoken.plainlist.space
WHERETOKEN_COOKIE_SECURE=1
```

Health: `GET /api/health` → `{"ok":true,"version":"0.7.0"}`. No DSN, paths, or env.

Logs: structured, request id, auth/sync/DB errors. Never log device tokens, OAuth secrets, or full sync bodies.

Sibling applications on the same deployment host are not edited by this work. Smoke after deploy: those hostnames still return themselves; whereToken no longer serves `sample/all.json` as the product.

`site/index.html` gains one CTA: **Open Web App** → `https://wheretoken.plainlist.space`. Demo / Download / GitHub / Docs stay. CLI footer, once Hosted is live, prefers the Web App URL; `--help` and README keep the Project Site. Until Hosted shows real data, do not point the default report at a demo hostname.

---

## 13. Migration

v0.6.4 users have no hosted account. There is no ledger to import.

Deploy sequence:

1. Create MySQL database `wheretoken` and a least-privilege application user with a freshly generated password stored only in the 0600 env file
2. Cross-compile `wheretoken-hosted` (`GOOS=linux GOARCH=amd64`)
3. Build `web` with `VITE_HOSTED=1` (and **not** `VITE_DEMO`)
4. Write `.env` 0600
5. Keep a copy of today's static demo as `wheretoken.plainlist.space.bak-demo`
6. `nginx -t` + reload; PM2 start; `GET /api/health`
7. OAuth callback smoke (after the GitHub App exists)

GitHub Pages, Homebrew, and `v0.6.4` GitHub Release are not part of this migrate.

---

## 14. Rollback

| Failure | Action |
| --- | --- |
| Hosted binary crash | Restart from the previous known-good build; nginx still serves the last Hosted dist or the pre-launch demo backup |
| Bad SPA | Point nginx `root` back at the previous release directory |
| Need the old public demo back | Restore the pre-launch demo backup (synthetic, honest as demo only) |
| API schema mistake | No user data of value in v1; `DROP DATABASE` is acceptable before public accounts exist |

Never: retag `v0.6.4`, force-push `main`, edit an unrelated sibling application to "fix" whereToken, or ship a Pages CNAME.

CLI rollback for users is `wheretoken update` staying on 0.6.4 until the 0.7.0 tag exists. `login`/`sync` are new commands; old binaries ignore them.

---

## 15. Test plan

TDD: failing test first for each package. `go test ./...`, `go vet ./...`, `cd web && npm test && npm run build && npx vue-tsc --noEmit`.

### Auth

- OAuth `state` mismatch → 400, no session
- Callback replay (consumed code / consumed state) → fail
- Session expiry → 401
- Logout revokes hash; cookie replay fails
- Unauthorized `/api/v1/dashboard/summary` → 401
- User A cannot GET/DELETE User B devices or usage (IDOR)

### Pairing

- Expired challenge
- Wrong display code
- Wrong device secret
- Challenge reuse after confirm
- User reject → CLI sees `denied`
- Revoke → sync 401

### Sync

- First sync
- Incremental (unchanged revision no-op)
- HTTP retry with same `idempotency_key`
- Stale revision → 409
- Invalid schema_version → 400
- Oversized body / too many rows → 413
- Unknown model → stored, estimate `unavailable` not `$0`
- Partial pricing (one category unlisted)
- Two devices, Cursor `account_global` 100M → hosted 100M
- Two devices, Claude `device_local` 50M + 30M → hosted 80M

### Web

- Logged out → `/login`
- Never synced → onboarding, not sample
- Device offline → last data + banner
- Fresh / stale sync copy
- 2×5 labels; no 排名
- Portrait states `none` / `insufficient` / `ok`
- 估价 modal
- Period today/7d/30d/all
- Source status strip
- Hosted build never requests `sample/*.json`

### Privacy

Construct a sync payload from a fixture scan that includes paths/JWTs in the **local** events and assert the upload JSON contains none of the forbidden keys.

### Regression

Local `httpapi` still refuses non-localhost. Community honesty tests still pass. `wheretoken update` tests still pass. Pages demo tests still pass.

---

## Package map (implementation)

```
cmd/wheretoken-hosted/          # server main; not goreleaser user archives
internal/hosted/
  http.go                       # public mux
  auth_oauth.go
  auth_session.go
  pair.go
  device.go
  sync.go
  dashboard.go                  # reconstruct summary JSON
  store.go                      # MySQL
internal/syncagg/               # daily×model from []event.UsageEvent (CLI + tests)
internal/credstore/             # Keychain / libsecret / WinCred / 0600 file
internal/cli/                   # CommandLogin, CommandLogout, CommandSync
web/src/api.ts                  # hosted fetch
web/src/hosted.ts
web/src/pages/Login.vue Pair.vue Devices.vue Privacy.vue
```

Do not put hosted routes on `internal/httpapi.NewMux*`.

Suggested commits (adjust if files couple):

```
feat(auth): add hosted account authentication
feat(device): add secure CLI device pairing
feat(sync): add privacy-safe usage synchronization
feat(web): add hosted dashboard mode
chore(deploy): deploy hosted whereToken app
docs: document hosted sync and privacy model
chore(release): v0.7.0
```

---

## Out of v0.7.0

Community Rank as a public board, social feed, teams, billing, public profiles, LLM portrait, raw prompt storage, server-side scanning, remote file browser, `sync --watch`, localhost bridge.

---

## GitHub OAuth App configuration

```
Application name:              whereToken
Homepage URL:                  https://wheretoken.plainlist.space
Authorization callback URL:    https://wheretoken.plainlist.space/api/v1/auth/github/callback
```

No extra scopes. Put Client ID / Secret only in the 0600 env file the hosted binary reads; never in the repository or in a web-served path.

Deploy-time task: create the MySQL database and application user described in [§13](#13-migration) before starting the hosted binary.

---

## Success

`wheretoken update && wheretoken login && wheretoken sync` then `https://wheretoken.plainlist.space` shows this machine's 总用量 / 模型 / 估价 / 用户画像, not `sample/all.json`. Project Site demo remains at GitHub Pages.
