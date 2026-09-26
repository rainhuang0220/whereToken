# Production baseline

Observed 2026-09-26 17:41–17:43 UTC from this environment. This file records only results that returned. It does not repeat an earlier audit.

## Hosted binary

`https://wheretoken.plainlist.space/api/health` returned HTTP 200 at `Sat, 26 Sep 2026 17:41:46 GMT`.

```json
{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}
```

`Server: nginx`. The body is the hosted Go health handler (`internal/hosted` `/api/health`), whose `version` is `resolveVersion` in `cmd/wheretoken-hosted`: the link-time value, or else `debug.ReadBuildInfo` main version.

In this clone, `9af013644f5caf52d3eb056886ad7b1e1894266c` is `docs: note the orphaned test database in the disk check`, committed `2026-09-24 02:11:26 +0800` (18:11:26 UTC), which matches the pseudo-version timestamp. `git describe --tags` on that commit is `v0.7.4-7-g9af0136`. The pseudo-version `v0.7.5-0.<timestamp>-<commit>` is Go's next-patch form when the newest ancestor tag is v0.7.4. That commit is an ancestor of the current `v0.7.5` tag (`d4cd75008e70fedf413f6c0bce2debcb5406da29`) and of the `v0.7.6` tag (`42f22dcdf85502937e063a8595846d78e35d691d`). Production is not the v0.7.6 release binary.

Process id, process owner, on-disk binary SHA, and startup command were not read. SSH is blocked (below).

## Database

BLOCKED. No SSH session and no production DSN. Schema version, `public_projections`, `public_presentations`, and `publish_jobs` were not queried on the host.

The public profile payload is not the database schema. `GET /api/v1/public-profile/rainhuang0220` returned HTTP 200 (205853 bytes) with `schema` `wheretoken.public-profile-live`, `schema_version` 1, snapshot `schema_version` 2. The JSON has no `verified_snapshot_id`, `verified_token_total`, or `verified_time` fields.

This agent VM has MariaDB 10.11.14 listening on `127.0.0.1:3306` and a pre-existing database named `wheretoken`. That database was not treated as production, was not dumped, and was not migrated. A separate empty database was used for the disposable migration cases and then dropped. See `engineering/release_acceptance_v0.7.7.md`.

## Public HTTP

DNS: `wheretoken.plainlist.space` is `175.24.134.228`.

| Request | Result |
| --- | --- |
| `GET /` | HTTP 200, `Server: nginx`, `Last-Modified: Sun, 06 Sep 2026 05:52:53 GMT`, 790-byte SPA shell (`/assets/index-CTggJBWx.js`, `/assets/index-CTwlroJs.css`) |
| `GET /healthz` | HTTP 200, same SPA HTML. nginx does not send this path to the Go health handler |
| `GET /api/v1/health`, `/api/version`, `/api/v1/meta`, `/api/v1/public/profile` | HTTP 404, body `404 page not found` (Go) |
| `GET /api/v1/session` | HTTP 401, body `unauthorized` |
| `GET /api/v1/dashboard/summary` | HTTP 401, body `unauthorized` |
| `PUT /api/v1/sync/batch` | HTTP 401, body `unauthorized` |
| `PUT /api/v1/sync/public-profile` | HTTP 401, body `unauthorized` |
| `GET /api/v1/account` | HTTP 405, body `method not allowed` |
| `POST /api/v1/auth/logout` | HTTP 204, empty body |
| `GET /api/v1/auth/github` | HTTP 302 to `https://github.com/login/oauth/authorize` |
| `GET /api/v1/public-profile/rainhuang0220` | HTTP 200 JSON, fields below |

Routing matches `docs/deployment.md`: nginx serves the SPA and reverse-proxies `/api/` to the hosted process. The loopback port, PM2 name, and env file were not confirmed on the host.

Anonymous public profile fields:

- `snapshot.snapshot_id`: `sha256:b8c90696a49ce62cdbeb620e2d2d4df99eca7cdb3b6f2d03c38cdb5b463d563d`
- `asset_revision` and presentation revision: `sha256:6720ba3e714ed5a521e3cfcad145b740610024eeaed52208c033cd2657b6d0cf`
- `presentation.public_palette`: `cobalt`
- `freshness`: `near_real_time`, source `hosted`, `updated_at` `2026-09-23T18:09:45Z`
- snapshot `generated_at` `2026-09-23T18:09:14Z`, `as_of_date` `2026-09-24`, `data_status` `partial`
- producer `wheretoken` `v0.7.5-0.20260923180341-d3c9bee0cc69`, which is commit `d3c9bee0cc695ca43b05a8c75bff3fa6daec32b4` (`v0.7.4-6-gd3c9bee`), an ancestor of the hosted commit above
- privacy: raw events false, models omitted, cost omitted

`https://rainhuang0220.github.io/whereToken/` returned HTTP 200 from `GitHub.com` at `Sat, 26 Sep 2026 17:42:28 GMT` (11282 bytes). `https://kiln.plainlist.space/` returned HTTP 200 from nginx.

No authenticated session was attempted. No owner credentials were present.

## Access

- `~/.ssh` contains only `known_hosts` (one line). No `config` and no private key.
- `SSH_AUTH_SOCK` was unset. `ssh-add -l` returned `Could not open a connection to your authentication agent.`
- `ssh -o BatchMode=yes -o ConnectTimeout=8` to `ubuntu@175.24.134.228` and `root@175.24.134.228` both returned `Permission denied (publickey)`.
- `/home/ubuntu/wheretoken` does not exist on this machine. `~/.kube` does not exist.
- `gh api user` returned HTTP 403 `Resource not accessible by integration`.
- `gh api repos/rainhuang0220/whereToken` permissions: `admin`, `maintain`, `pull`, `push`, and `triage` are all false.
- `git push --dry-run origin cursor/release-v077-3e3c` returned `Everything up-to-date` before this baseline was committed. That is not a merge to `main`.

## Rollback point

The running service is the binary whose health string is `v0.7.5-0.20260923181126-9af013644f5c` (commit `9af013644f5caf52d3eb056886ad7b1e1894266c`). This environment does not have that binary file, its SHA256, its process id, or a database dump. Those cannot be restored from here.

Latest GitHub Release is still `v0.7.6` (published `2026-09-26T05:14:28Z`). Tag object `refs/tags/v0.7.6` is `3f24faa83c9f601f83aa9296cee76382023f78fa` and peels to `42f22dcdf85502937e063a8595846d78e35d691d`. That tag was not moved. It is a newer source commit than the running binary, so it is not a byte-for-byte rollback of production.

`origin/main` is `b4c9f81252656c0524208db019771344dfda2658`. No v0.7.7 tag exists. `gh release view v0.7.7` returned `release not found`.
