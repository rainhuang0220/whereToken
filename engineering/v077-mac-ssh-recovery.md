# v0.7.7 Mac SSH recovery

Date: 2026-10-01. Target: `ubuntu@175.24.134.228`. Expected production key: `/Users/rainhuang/Desktop/whereToken/id_rsa`.

This session was re-checked on the machine that is actually running. It is still the Linux cloud agent VM. Mac paths are absent. No SSH login was attempted. Nothing was deployed. PM2, the database, nginx, and application code were not changed. No key was generated.

## Environment

| Check | Result |
| --- | --- |
| `uname -a` | `Linux cursor 6.12.94+ #1 SMP PREEMPT_DYNAMIC Thu Sep 24 16:04:37 UTC 2026 x86_64 x86_64 x86_64 GNU/Linux` |
| `sw_vers` | Not present |
| OS | Ubuntu 24.04.4 LTS (`/etc/os-release`) |
| `$HOME` | `/home/ubuntu` |
| `pwd` | `/workspace` |
| user | `ubuntu` |
| `/Users` | Absent |
| `/Users/rainhuang` | Absent |
| `/Users/rainhuang/Desktop/whereToken/id_rsa` | Absent |
| `~/Desktop`, `~/Downloads`, `~/Documents` | Absent |

## SSH

FAILED

key path: none. The production key was not on this machine.

connection: not attempted. Phases 2–7 were skipped.

## Search

Searched once under `$HOME` and `/workspace` for `id_rsa*`, `*.pem`, and `*.key`, pruning `node_modules`, `.git`, `go/pkg/mod`, `vendor`, `testdata`, noVNC fixtures, and certifi.

| Check | Result |
| --- | --- |
| `~/.ssh` | Absent. No `/home/ubuntu/.ssh`. |
| `/home/ubuntu/.ssh/wheretoken_tencent_ed25519` | Absent on this VM. That path, when it existed on an earlier Linux session, was a generated key and is not the production key. It was not used and is not FOUND. |
| Private keys under `$HOME` and `/workspace` after prune | None |

The only `*.pem` names under `/home/ubuntu` sit in `go/pkg/mod` toolchain testdata. Those trees were pruned and were not opened or tested.

## Deployment

old version: not collected. SSH did not run.

new version: not deployed.

migration: not run.

health: not re-fetched. Last known body from the operator, before this session: `{"status":"ok","version":"v0.7.5-0.20260923181126-9af013644f5c"}`. This session does not claim that body is still current.

## Wall

README: not checked.

SVG: not checked.

snapshot: not published. `verified_snapshot_id`, `verified_raw_total`, and `verified_publish_time` were not touched.

status: not verified. Hosted v0.7.7 was not deployed.

## Next action

STOP. This Cursor session is still Linux (`$HOME=/home/ubuntu`). `/Users/rainhuang` does not exist here, so the production key at `/Users/rainhuang/Desktop/whereToken/id_rsa` cannot be read from this agent.

Operator step: run the deploy from the Mac session where that `id_rsa` file is present, or copy that existing private key into a session that can see it. Do not generate a replacement key for this VM. Do not treat a cloud-agent ed25519 key as production access.
