# Production access blocked

Observed Monday 28 September 2026. This session did not log in to the host, did not copy a binary, did not read MySQL, and did not restart anything. Deployment stopped here.

## Missing permission

SSH public-key login to the production host. This machine has no SSH private key and no agent, so SSH was not attempted. A previous session already recorded `Permission denied (publickey)` (see `engineering/PRODUCTION_ACCESS_BLOCKED.md`, 2026-09-26). That denial was not repeated.

## What is known, and what is missing

| Piece | Found in the repo or on the public internet | Missing on this machine / not observed on the host |
| --- | --- | --- |
| Hostname | `wheretoken.plainlist.space` | — |
| Address | `175.24.134.228` (`getent hosts` on 2026-09-28) | — |
| SSH user | `ubuntu`, from `docs/architecture/hosted-web-v070.md` (`ubuntu@175.24.134.228`) | A shell. `root` was denied in the 2026-09-26 note and was not tried again. |
| SSH key | The ADR names `id_rsa`. It says `id_ed25519` was rejected during the 2026-09-04 discovery. | The key file. `~/.ssh` does not exist. `SSH_AUTH_SOCK` is unset. `ssh-add -l` returned `Could not open a connection to your authentication agent.` No `id_rsa`, `id_ed25519`, or deploy `*.pem` was found outside Go module test certificates. |
| Deployment account | `ubuntu`, env file `/home/ubuntu/wheretoken/shared/.env` mode `0600` (`docs/deployment.md`, ADR section 12, `scripts/hosted.env.example`) | The file, the directory, and the process owner uid. `/home/ubuntu/wheretoken` is not on this agent machine. |
| PM2 | Process name `wheretoken-hosted`, bind `127.0.0.1:3400`. nginx `location /api/` proxies to that loopback address. | PM2 owner, pid, binary path, and the ecosystem file. None of those are in the repo. The public `Server: nginx` header only confirms the TLS front. |
| Database | Name `wheretoken` on `127.0.0.1`. Env var `WHERETOKEN_MYSQL_DSN`. The ADR names MySQL user `wheretoken`. `scripts/hosted.env.example` names user `wheretoken_app`. The 2026-09-04 ADR also said MySQL 5.7.43 and, at that time, no `wheretoken` database yet. | Live user, password, data directory, and schema. The DSN was not read. This session did not open the agent VM's database and does not treat it as production. |
| GitHub deploy | Release workflow `.github/workflows/release.yml` publishes the CLI with goreleaser. It does not SSH. Secret names in that workflow file: `MACOS_SIGN_P12`, `MACOS_SIGN_PASSWORD`, `MACOS_NOTARY_KEY`, `MACOS_NOTARY_KEY_ID`, `MACOS_NOTARY_ISSUER_ID`, `GITHUB_TOKEN`. | `gh api repos/rainhuang0220/whereToken/actions/secrets` returned HTTP 403 `Resource not accessible by integration`. Values were not requested. There is no hosted-deploy workflow. |
| GitHub token here | `gh` is logged in as the `cursor` app. `repos/rainhuang0220/whereToken` permissions are `admin`, `maintain`, `pull`, `push`, and `triage` all false. | This token cannot deploy the host and cannot push the profile repository. |

`scripts/build-hosted.sh` is the hosted build. It cross-compiles `cmd/wheretoken-hosted` for `linux/amd64` and builds the SPA with `VITE_HOSTED=1` and an empty `VITE_DEMO`. The ADR says the server has no Go toolchain, so that script runs off the server. It was not run.

## Health response

`GET https://wheretoken.plainlist.space/api/health` at `Mon, 28 Sep 2026 08:10:43 GMT`.

```text
HTTP/1.1 200 OK
Server: nginx
Date: Mon, 28 Sep 2026 08:10:43 GMT
Content-Type: application/json; charset=utf-8
Content-Length: 65
```

```json
{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}
```

That body is still the pre-v0.7.7 process. `9af013644f5c` is `9af013644f5caf52d3eb056886ad7b1e1894266c` (`docs: note the orphaned test database in the disk check`, `2026-09-24 02:11:26 +0800`). `git describe --tags` of that commit is `v0.7.4-7-g9af0136`. It is an ancestor of `v0.7.7`. It is not the `v0.7.7` tag.

## Required operator action

Install this operator's SSH public key on the `ubuntu` account at `175.24.134.228`, using the `id_rsa` pairing the ADR records, or start a session that already has that private key loaded. Do not paste a private key into chat, the repo, or this agent.

After `ssh ubuntu@175.24.134.228` opens a shell, stop and take the backup below before replacing the binary. Startup of `cmd/wheretoken-hosted` calls `Store.Migrate`. A start is the migration.

## Commands after access is restored

Run the record and backup commands on the host. Do not print `.env` or the DSN. The pid, binary path, and MySQL data directory were not observed from here, so the commands discover them. The process name, env path, listen address, and database name are the ones in the docs.

Record the running process:

```bash
pm2 pid wheretoken-hosted
pm2 describe wheretoken-hosted
pid="$(pm2 pid wheretoken-hosted)"
tr '\0' ' ' < /proc/"$pid"/cmdline; echo
readlink -f /proc/"$pid"/exe
sha256sum "$(readlink -f /proc/"$pid"/exe)"
ss -ltnp | grep 3400
```

Back up the binary, the env file, and the nginx SPA root that is actually configured. Documented SPA root: `/www/wwwroot/wheretoken-releases/<sha>/`. Copy the root nginx is using. Do not print `.env`.

```bash
install -d -m 0700 /home/ubuntu/wheretoken/shared/pre-v077
cp -a "$(readlink -f /proc/$(pm2 pid wheretoken-hosted)/exe)" \
  /home/ubuntu/wheretoken/shared/pre-v077/wheretoken-hosted
cp -a /home/ubuntu/wheretoken/shared/.env \
  /home/ubuntu/wheretoken/shared/pre-v077/.env
chmod 0600 /home/ubuntu/wheretoken/shared/pre-v077/.env
# copy the live nginx root into pre-v077/web as well
sha256sum /home/ubuntu/wheretoken/shared/pre-v077/wheretoken-hosted \
  /home/ubuntu/wheretoken/shared/pre-v077/.env
stat -c '%n %s %y' /home/ubuntu/wheretoken/shared/pre-v077/wheretoken-hosted \
  /home/ubuntu/wheretoken/shared/pre-v077/.env
```

Back up MySQL before the new binary starts. There is no dump script in the repo. Database name in the docs is `wheretoken`. Use the user inside the live `WHERETOKEN_MYSQL_DSN` (the ADR says `wheretoken`; the example file says `wheretoken_app`). Write a mode `0600` client defaults file on the host, run the dump, then delete that defaults file. Do not put the password on the command line or in git.

```bash
mysqldump --defaults-extra-file=/home/ubuntu/wheretoken/shared/pre-v077/mysql-client.cnf \
  --single-transaction --routines --triggers wheretoken \
  > /home/ubuntu/wheretoken/shared/pre-v077/wheretoken.sql
chmod 0600 /home/ubuntu/wheretoken/shared/pre-v077/wheretoken.sql
rm -f /home/ubuntu/wheretoken/shared/pre-v077/mysql-client.cnf
sha256sum /home/ubuntu/wheretoken/shared/pre-v077/wheretoken.sql
stat -c '%n %s %y' /home/ubuntu/wheretoken/shared/pre-v077/wheretoken.sql
```

Build off the server, from the `v0.7.7` tag. The tag object `61850b5c1ab872d5b94972df6b7248648e7d1666` peels to `6948f7522a0a98d4f7d2619583e1b717b9363162`. `scripts/build-hosted.sh` sets `VITE_HOSTED=1` and leaves `VITE_DEMO` empty. Do not build with `VITE_DEMO=1`.

```bash
git fetch origin tag v0.7.7
git checkout v0.7.7
git rev-parse 'v0.7.7^{commit}'
scripts/build-hosted.sh
```

That script's `go build` ldflags are `-s -w` only. It does not pass `-X main.version`. `resolveVersion` in `cmd/wheretoken-hosted/main.go` then falls through to build info, and a local build reports `dev`. Health must be the literal `v0.7.7` string. Rebuild only the binary with the ldflag that variable already reads, after the script has produced the SPA:

```bash
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath \
  -ldflags "-s -w -X main.version=v0.7.7" \
  -o dist/hosted/wheretoken-hosted ./cmd/wheretoken-hosted
go version -m dist/hosted/wheretoken-hosted
```

`vcs.revision` must be `6948f7522a0a98d4f7d2619583e1b717b9363162`. Copy `dist/hosted/wheretoken-hosted` onto the path PM2 executes. Copy `dist/hosted/web` to `/www/wwwroot/wheretoken-releases/6948f7522a0a98d4f7d2619583e1b717b9363162/` and point the nginx `root` at that directory. Keep the previous directory for rollback.

```bash
nginx -t && nginx -s reload
pm2 restart wheretoken-hosted
pm2 describe wheretoken-hosted
```

`pm2 restart wheretoken-hosted` is the migration. `Migrate` runs before listen (`CREATE TABLE IF NOT EXISTS`, `ensureColumns`, `backfillVerifiedIdentity`). Do not reset the verified watermark, do not write unknown totals as zero, and do not overwrite GitHub publication history. If the process exits before listen, roll back to the `pre-v077` binary and dump. Do not bind `0.0.0.0:3400`.

Health, from anywhere:

```bash
curl -fsS -D - https://wheretoken.plainlist.space/api/health
```

Continue to a wall publication only when that body is:

```json
{"status":"ok","version":"v0.7.7"}
```

The profile repository is `rainhuang0220/rainhuang0220`, branch `main`, files `README.md`, `wheretoken/preview-light.svg`, and `wheretoken/preview-dark.svg`. A hosted PUT is not that check. Read those three files back from GitHub. They must share one snapshot id, one cache key, and one date before the wall is verified.
