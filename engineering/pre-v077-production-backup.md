# Pre-v0.7.7 production backup

Observed 2026-09-26, re-checked after `git fetch origin main cursor/release-v077-3e3c cursor/unified-github-wall-3e3c`. This file records only checks that returned a result. No backup was written.

## BLOCKED

Production backup is BLOCKED. MySQL on the production host was not read. The current binary, the PM2 process, and the deployment env file were not copied. No backup path, checksum, or process id was observed, so none is recorded.

Checked:

- DNS: `wheretoken.plainlist.space` resolves to `175.24.134.228`, the address in `docs/architecture/hosted-web-v070.md`.
- `~/.ssh` exists and contains only `known_hosts` (one hashed line). There is no `~/.ssh/config` and no private key (`id_rsa`, `id_ed25519`, or otherwise) under the home directory.
- `SSH_AUTH_SOCK` was unset. `/tmp/ssh-HwsRV4aWXkKV/agent.2621` refused the connection. `/tmp/ssh-ravwmbCNfOyz/agent.1640` answered and reported no identities.
- `ssh -o BatchMode=yes -o ConnectTimeout=8` to `ubuntu@175.24.134.228` and `root@175.24.134.228` both returned `Permission denied (publickey)`.
- No `WHERETOKEN_MYSQL_DSN`, deploy token, or GitHub publish token was present in the environment. No `.env` file was found under `/home/ubuntu` at depth 3. `/home/ubuntu/wheretoken` does not exist on this machine. `~/.kube` does not exist. `brew` is not installed. Docker is not on `PATH`.
- The `gh` credential is a GitHub App installation token. `X-Accepted-Github-Permissions` on `rainhuang0220/whereToken` and `rainhuang0220/homebrew-wheretoken` is `metadata=read`. Repository permission objects report `admin`, `maintain`, `push`, `pull`, and `triage` all false. `GET /user` returned 403 `Resource not accessible by integration`.

Missing permission: an SSH login as the host user that can read the PM2 process `wheretoken-hosted`, `/home/ubuntu/wheretoken/shared/.env`, and the MySQL data directory. Public HTTP below is not that access.

## Public HTTP

`https://wheretoken.plainlist.space/api/health` at `Sat, 26 Sep 2026 16:15:10 GMT`:

- `HTTP/1.1 200 OK`
- `Server: nginx`
- `Content-Type: application/json; charset=utf-8`
- `Content-Length: 65`
- `Strict-Transport-Security: max-age=86400`
- body: `{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}`

`https://wheretoken.plainlist.space/` at `Sat, 26 Sep 2026 16:15:11 GMT`:

- `HTTP/1.1 200 OK`
- `Server: nginx`
- `Content-Type: text/html`
- `Content-Length: 790`
- `Last-Modified: Sun, 06 Sep 2026 05:52:53 GMT`
- title `whereToken`
- script `/assets/index-CTggJBWx.js` (200, 265907 bytes). That file does not contain `演示数据` or `sample/all.json`.

`GET /api/v1/session` at `Sat, 26 Sep 2026 16:15:12 GMT` returned `401` with body `unauthorized`. No login was attempted.

`GET /api/v1/public-profile/rainhuang0220` at `Sat, 26 Sep 2026 16:15:14 GMT` returned `200`. Fields read from that JSON:

| Field | Value |
| --- | --- |
| schema | `wheretoken.public-profile-live` |
| schema_version | `1` |
| owner | `rainhuang0220` |
| data_revision / snapshot.snapshot_id | `sha256:b8c90696a49ce62cdbeb620e2d2d4df99eca7cdb3b6f2d03c38cdb5b463d563d` |
| snapshot.as_of_date | `2026-09-24` |
| snapshot.generated_at | `2026-09-23T18:09:14Z` |
| snapshot.data_status | `partial` |
| snapshot.schema_version | `2` |
| freshness.source | `hosted` |
| freshness.mode | `near_real_time` |
| freshness.updated_at | `2026-09-23T18:09:45Z` |
| presentation.public_palette | `cobalt` |
| asset_revision | `sha256:6720ba3e714ed5a521e3cfcad145b740610024eeaed52208c033cd2657b6d0cf` |

The JSON has no `wall` object and no `verified_snapshot_id`. This is the public API response, not a database row.

## Version string and this clone

The health body is the only version the process exposed. In this clone, `9af013644f5c` is `9af013644f5caf52d3eb056886ad7b1e1894266c` (`docs: note the orphaned test database in the disk check`, committer time `2026-09-24T02:11:26+08:00`). `git describe --tags` of that commit is `v0.7.4-7-g9af0136`. That commit is an ancestor of the `v0.7.5` and `v0.7.6` tag commits. Those tags are not ancestors of it. The host binary file was not opened.

## This machine's MariaDB

MariaDB is listening on `127.0.0.1:3306`. `sudo mysql` via `/var/run/mysqld/mysqld.sock` reported `10.11.14-MariaDB-0ubuntu0.24.04.1`. Schema names returned: `information_schema`, `mysql`, `performance_schema`, `sys`, `wheretoken`. Table rows were not read. This server is not `175.24.134.228`. It was not backed up, migrated, or used as a test target in this session.

## Documented deploy path, not observed live

`docs/deployment.md` and `docs/architecture/hosted-web-v070.md` describe nginx on `wheretoken.plainlist.space`, `/api/` proxied to `127.0.0.1:3400`, PM2 name `wheretoken-hosted`, and env file `/home/ubuntu/wheretoken/shared/.env`. nginx is the only part confirmed, by the `Server` header. The loopback port, PM2, env file, and database were not confirmed on the host.
