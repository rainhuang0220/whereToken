# SSH key discovery

Date: 2026-09-28. Target: `ubuntu@175.24.134.228` (`wheretoken.plainlist.space`). This note is access recovery only. Nothing was deployed. PM2, the database, nginx, and application code were not changed.

## SSH Key Discovery

NOT FOUND

No existing operator private key was on this machine. A new ed25519 key was generated only after that search. It is outside the git repo. It was not committed.

## Search

| Check | Result |
| --- | --- |
| `$HOME` | `/home/ubuntu` |
| `~/.ssh` before this note | Absent. No `/home/ubuntu/.ssh`. |
| `~/.ssh/config` | Did not exist. No Host entries and no IdentityFile paths. |
| `SSH_AUTH_SOCK` | Unset. |
| `ssh-add -l` with no sock | `Could not open a connection to your authentication agent.` |
| Agent socket `/tmp/ssh-Ph46bvUmUhws/agent.964` | Reached. `ssh-add -l` reported no identities. |
| Agent socket `/tmp/ssh-RW30Eybea4dW/agent.2478` | Connection refused. |
| `/root/.ssh` | Directory mode `700`, empty. Listed with `sudo -n`. No key files. |
| Name `rainhuang` | No directory or file. `~/rainhuang`, Desktop, Documents, Downloads, and Projects are absent. A filesystem name search (pruning `/proc`, `/sys`, `/dev`, `/usr`, `node_modules`, `.git`, `go/pkg/mod`, `vendor`) found nothing. |

Directories actually scanned for key filenames and for private-key headers (`OPENSSH PRIVATE KEY`, `BEGIN PRIVATE KEY`, PuTTY): `/home`, `/workspace`, `/opt`, `/opt/cursor`, `/tmp`, `/cursor`, `/usr/local`, `/root`, `/etc`, `/var`, `/srv`, `/mnt`, `/media`. Pruned: `node_modules`, `.git`, `go/pkg/mod`, `vendor`, `.npm`, `.nvm`, `.cache`, `testdata`.

Filename patterns: `*.pem`, `*.key`, `*.ppk`, `id_rsa*`, `id_ed25519*`, `id_ecdsa*`, and names containing `ssh`, `ubuntu`, `tencent`, `cloud`, `server`, `rainhuang`. A `*tx*` name pass under home, workspace, `/opt`, `/tmp`, and `/cursor` only hit module caches, logs, and source text, not keys.

## Candidate keys

None.

Skipped, not probed, not chmodded:

| Path | Why skipped |
| --- | --- |
| `/usr/local/novnc/websockify-0.10.0/tests/fixtures/private.pem` | noVNC test fixture. `file`: PEM RSA private key. Mode `644`. |
| `/usr/local/novnc/websockify-0.10.0/tests/fixtures/public.pem` | Test fixture public half. |
| `/usr/local/novnc/websockify-0.10.0/tests/fixtures/symmetric.key` | Test fixture, not an SSH key. |
| `/usr/local/lib/python3.12/dist-packages/certifi/cacert.pem` | Public CA bundle. |
| `/home/ubuntu/.gnupg/private-keys-v1.d` | Empty directory. Not an SSH key. |
| Go module `testdata` and `go/pkg/mod` | Dependency and test trees. Not opened. |

## Server access test

FAIL (new key not installed on the server)

Private key path: `/home/ubuntu/.ssh/wheretoken_tencent_ed25519`

Permissions: directory `/home/ubuntu/.ssh` was created and set to `700`. `ssh-keygen` wrote the private key as mode `600` (`-rw-------`). No extra chmod was applied to the key file. Public key mode `644`.

`file`: OpenSSH private key. Public half: OpenSSH ED25519 public key.

Public fingerprint (`ssh-keygen -lf`):

```text
256 SHA256:CZ931Msrr4Zwhc8BkDR7SoInn8eMPWIS5fUAra36nv8 wheretoken-tencent-ubuntu@175.24.134.228 (ED25519)
```

Public key line:

```text
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBvhHYAFSLnBel4aSx/SGxMpqtJbkpV2k5Ue0p5UR0qD wheretoken-tencent-ubuntu@175.24.134.228
```

One probe. It was not retried.

```bash
ssh -i /home/ubuntu/.ssh/wheretoken_tencent_ed25519 -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 ubuntu@175.24.134.228 'echo ok'
```

Result: `ubuntu@175.24.134.228: Permission denied (publickey).` Exit 255. The client recorded the server host key in the local `known_hosts` because `StrictHostKeyChecking=accept-new` was set. The server was not modified.

## Next action

STOP. Do not deploy from this task.

Operator step: add this public key to `ubuntu@175.24.134.228` in the Tencent Cloud console, or to that account's `~/.ssh/authorized_keys`. Do not paste the private key anywhere.

```text
ssh-ed25519 AAAAC3NzaC1lZDI1NTE5AAAAIBvhHYAFSLnBel4aSx/SGxMpqtJbkpV2k5Ue0p5UR0qD wheretoken-tencent-ubuntu@175.24.134.228
```

After it is installed, re-run:

```bash
ssh -i /home/ubuntu/.ssh/wheretoken_tencent_ed25519 -o BatchMode=yes -o IdentitiesOnly=yes -o StrictHostKeyChecking=accept-new -o ConnectTimeout=10 ubuntu@175.24.134.228 'echo ok'
```

A second key was not generated. This session did not install the public key on the server.
