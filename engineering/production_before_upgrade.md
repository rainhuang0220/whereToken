# Production before upgrade

Observed 2026-09-26. This file records only checks that returned a result. It is the before-state for a v0.7.7 hosted upgrade that did not run.

## BLOCKED

Production host access is BLOCKED. The hosted database was not read. No process id, binary path, PM2 status, or environment file was observed on the server.

Checked:

- `~/.ssh` does not exist. `~/.kube` does not exist. `/home/ubuntu/wheretoken` does not exist on this machine.
- No `WHERETOKEN_MYSQL_DSN`, deploy token, or GitHub publish token was present in the environment.
- An SSH agent key was loaded. `ssh -o BatchMode=yes` to `ubuntu@175.24.134.228` and `root@175.24.134.228` (the address in `docs/architecture/hosted-web-v070.md`) both returned `Permission denied (publickey)`.
- Docker is not installed. There is no kube context.

Missing permission: SSH (or another shell) as the host user that can read the PM2 process, `/home/ubuntu/wheretoken/shared/.env`, and the MySQL schema. Do not treat the public HTTP responses below as that access.

## Public HTTP

`https://wheretoken.plainlist.space/api/health` at `Sat, 26 Sep 2026 15:54:56 GMT`:

- `HTTP/1.1 200 OK`
- `Server: nginx`
- `Content-Type: application/json; charset=utf-8`
- `Strict-Transport-Security: max-age=86400`
- body: `{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}`

`https://wheretoken.plainlist.space/` at the same minute:

- `HTTP/1.1 200 OK`
- `Server: nginx`
- `Content-Type: text/html`
- `Content-Length: 790`
- `Last-Modified: Sun, 06 Sep 2026 05:52:53 GMT`
- title `whereToken`
- script `/assets/index-CTggJBWx.js` (200, 265907 bytes). That file does not contain `演示数据` or `sample/all.json`.

`GET /api/v1/session` returned `401`. No login was attempted, so a working authenticated session was not observed.

`GET /api/v1/public-profile/rainhuang0220` returned `200` (205853 bytes). Fields read from that JSON, without token totals:

| Field | Value |
| --- | --- |
| schema | `wheretoken.public-profile-live` |
| schema_version | `1` |
| owner | `rainhuang0220` |
| snapshot_id / data_revision | `sha256:b8c90696a49ce62cdbeb620e2d2d4df99eca7cdb3b6f2d03c38cdb5b463d563d` |
| snapshot.as_of_date | `2026-09-24` |
| snapshot.generated_at | `2026-09-23T18:09:14Z` |
| snapshot.data_status | `partial` |
| snapshot.schema_version | `2` |
| freshness.source | `hosted` |
| freshness.mode | `near_real_time` |
| freshness.updated_at | `2026-09-23T18:09:45Z` |
| presentation.public_palette | `cobalt` |
| asset_revision | `sha256:6720ba3e714ed5a521e3cfcad145b740610024eeaed52208c033cd2657b6d0cf` |

The JSON has no `wall` object and no `verified_snapshot_id`. README and SVG bytes were not fetched from GitHub.

## Version string and this clone

The health body is the only version the process exposed. In this clone, `9af013644f5c` resolves to `9af013644f5caf52d3eb056886ad7b1e1894266c` (`docs: note the orphaned test database in the disk check`, committer time `2026-09-24 02:11:26 +0800`, which is `2026-09-23 18:11:26 UTC`). `git describe --tags` of that commit is `v0.7.4-7-g9af0136`. `v0.7.5` and `v0.7.6` are not ancestors of it. The host binary file was not opened, so this is not a deployed-SHA confirmation from the machine.

## Local MariaDB on this VM

This machine's MariaDB was listening on `127.0.0.1:3306` (socket `/var/run/mysqld/mysqld.sock`, client path `/run/mysqld/mysqld.sock` was absent). Version `10.11.14-MariaDB-0ubuntu0.24.04.1`. Root over TCP was denied. `sudo mysql` via the socket worked. That is not the host at `175.24.134.228`.

A database named `wheretoken` was already present. Its table list includes the hosted tables, and `verified_snapshot_id`, `verified_total_tokens`, and `pending_due_at` already exist. Row contents were not read. It was not migrated, backed up, or dropped in this session. Disposable test databases were separate and were removed afterward.

## Documented deploy path, not observed live

`docs/deployment.md` and `docs/architecture/hosted-web-v070.md` describe nginx on `wheretoken.plainlist.space`, `/api/` proxied to `127.0.0.1:3400`, PM2 name `wheretoken-hosted`, and env file `/home/ubuntu/wheretoken/shared/.env`. nginx is the only part confirmed, by the `Server` header. The loopback port, PM2, and env file were not confirmed.
