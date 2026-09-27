# Production access blocked

Observed 2026-09-26 in the same session as `GET https://wheretoken.plainlist.space/api/health` (`Date: Sat, 26 Sep 2026 19:12:46 GMT`). SSH is still denied. Nothing on the host was changed.

## Host

| Item | Value |
| --- | --- |
| Hostname | `wheretoken.plainlist.space` |
| Address | `175.24.134.228` (`getent hosts`) |
| SSH users tried | `ubuntu@175.24.134.228`, `root@175.24.134.228` |
| Key the deploy notes name | `docs/architecture/hosted-web-v070.md` records SSH as `ubuntu@175.24.134.228` with `id_rsa`, and records that `id_ed25519` was rejected in that discovery. This session did not copy or open a private key. |

This machine's `~/.ssh` contains only `known_hosts`. There is no `config` and no private key file. `SSH_AUTH_SOCK` was unset. `ssh-add -l` returned `Could not open a connection to your authentication agent.`

## What the repo says the deploy is

From `docs/deployment.md` and `docs/architecture/hosted-web-v070.md` section 12. Live confirmation is only the nginx `Server` header and the health JSON.

| Piece | Documented | Observed this session |
| --- | --- | --- |
| TLS front | nginx (existing aaPanel), `server_name wheretoken.plainlist.space` | `Server: nginx` on the health response |
| API proxy | `location /api/` → `http://127.0.0.1:3400` | Not observed. The public `/api/health` JSON is the hosted handler. |
| SPA root | `/www/wwwroot/wheretoken-releases/<sha>/` | Not observed |
| Process manager | PM2 name `wheretoken-hosted`, bind `127.0.0.1:3400` | Not observed. No systemd unit and no launchd plist for this process are in the repo. launchd in the product docs is the macOS client agent for `profile refresh watch`. |
| Env file | `/home/ubuntu/wheretoken/shared/.env`, mode `0600` | Not observed. `/home/ubuntu/wheretoken` does not exist on this agent machine. |
| Database | `WHERETOKEN_MYSQL_DSN`: user `wheretoken`, database `wheretoken`, host `127.0.0.1` | Not observed. The on-disk MySQL data directory is not named in the docs. Unknown. The ADR's 2026-09-04 discovery said MySQL 5.7.43 and, at that time, no `wheretoken` database yet. That is not a current schema reading. |

`scripts/build-hosted.sh` is the hosted build. It cross-compiles `cmd/wheretoken-hosted` for `linux/amd64` and builds the SPA with `VITE_HOSTED=1` and an empty `VITE_DEMO`. The ADR says the server has no Go toolchain, so that script runs off the server.

## Missing permission

Both probes used `ssh -o BatchMode=yes -o ConnectTimeout=8 -o PreferredAuthentications=publickey -o PasswordAuthentication=no`.

```text
ubuntu@175.24.134.228: Permission denied (publickey).
root@175.24.134.228: Permission denied (publickey).
```

Exit status 255 for each. No banner, no password prompt, no shell.

Public health in that same window, HTTP 200:

```json
{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}
```

That response is not host login. Process id, binary path, startup command, env file, and database were not read.

## Owner action

Add this operator's SSH public key to the `ubuntu` account on `175.24.134.228` (the user and `id_rsa` pairing the ADR records), or start a session that already has that key loaded. Do not paste a private key into chat, the repo, or this agent. After login works, stop before changing production until the backup in `engineering/v0.7.7_operator_runbook.md` is on disk.

GitHub repository permissions on this token are a separate gap (`push: false`, `metadata=read`). They do not grant this SSH login, and this SSH login does not grant merging pull request #10.

## After access is granted

These record and backup commands are operator commands. The repo does not contain a PM2 ecosystem file, a `mysqldump` script, or a host path for the running binary. The process name, env path, listen address, and database name below are the ones in the docs. The pid, binary path, and datadir were not observed, so none are filled in.

Record the running process before replacing anything:

```bash
pm2 pid wheretoken-hosted
pm2 describe wheretoken-hosted
pid="$(pm2 pid wheretoken-hosted)"
tr '\0' ' ' < /proc/"$pid"/cmdline; echo
readlink -f /proc/"$pid"/exe
sha256sum "$(readlink -f /proc/"$pid"/exe)"
ss -ltnp | grep 3400
```

Copy the binary, the env file, and the nginx root that is actually configured. Do not print `.env`.

```bash
install -d -m 0700 /home/ubuntu/wheretoken/shared/pre-v077
cp -a "$(readlink -f /proc/$(pm2 pid wheretoken-hosted)/exe)" \
  /home/ubuntu/wheretoken/shared/pre-v077/wheretoken-hosted
cp -a /home/ubuntu/wheretoken/shared/.env \
  /home/ubuntu/wheretoken/shared/pre-v077/.env
chmod 0600 /home/ubuntu/wheretoken/shared/pre-v077/.env
# SPA root is documented as /www/wwwroot/wheretoken-releases/<sha>/
# Copy the root nginx is using. The live root was not observed from here.
```

Backup MySQL before the new binary starts. `cmd/wheretoken-hosted` calls `Store.Migrate` on startup, so a start is the migration. There is no dump script in the repo. Use the `wheretoken` database from `WHERETOKEN_MYSQL_DSN` without putting the password on the command line or in a log. Write the dump under the `0700` directory above. Record the dump path and its sha256. A dump path was not observed because the backup was not taken.

Rollback, after that backup exists, is in the operator runbook: restore this binary, this `.env`, and this dump, point nginx back at the copied SPA root, then `nginx -t` and `pm2 restart wheretoken-hosted`. Health must return `v0.7.5-0.20260923181126-9af013644f5c` again.
