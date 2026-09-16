# Security

## Supported versions

Only the latest tagged release receives fixes. whereToken is Alpha; there is no long-term-support branch.

## Reporting a vulnerability

Use GitHub's private vulnerability reporting for this repository: **Security** tab → **Report a vulnerability**. This opens a private advisory visible only to maintainers and does not require sharing details in a public issue. If that option is unavailable, open a regular issue asking for a private contact channel — do not include exploit details, logs, or secrets in that issue.

Do not paste `~/.config`, `%APPDATA%`, session files, or hosted device tokens into issues or advisories. Redact before sharing.

Expect an initial response within a few days. There is no bug-bounty program.

## Local scanner (`wheretoken`, `wheretoken serve`)

whereToken reads **local** coding-agent ledgers on the machine that runs it. Local analytics do not leave the machine; local-first remains the core. `wheretoken serve` binds **127.0.0.1** only and sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`. `/api/scan` and `/api/summary` refuse a non-localhost `Host` (DNS rebinding). The CLI never prints JWTs, access tokens, API keys, `Authorization`, `Cookie`, or `Set-Cookie` values. `wheretoken scan --json` and `/api/scan` run the same redaction. If a source is degraded (Trae encrypted login, Cursor signed out), it says so in a footnote instead of dumping secrets. The dashboard does not load Google Fonts or other third-party assets.

## Hosted sync (`wheretoken login`, `wheretoken sync`, `wheretoken.plainlist.space`)

This is a separate, explicitly opt-in surface from the local scanner. Logging in pairs the device via GitHub OAuth (identity only, no `repo`/`workflow` scopes; the GitHub access token is discarded immediately after fetching id/login/avatar). `wheretoken sync` uploads privacy-reduced daily-by-model **aggregates** — never raw events, prompts, code, paths, workspace names, session/request ids, or credentials; see the README privacy table and [`docs/architecture/hosted-web-v070.md`](docs/architecture/hosted-web-v070.md) for the exact field list and deletion semantics. Device credentials are stored in the OS keychain when available, otherwise a `0600` local file. `wheretoken logout` revokes the device server-side and deletes the local credential; deleting synced data or the account (hosted Settings → Privacy) removes the corresponding server-side rows. Cookie-authenticated mutations on the hosted API require the `X-CSRF-Token` header; a cookie alone is not accepted.

Report a suspected hosted-service compromise (unexpected device, unauthorized sync, or a credential-handling defect) the same way as any other vulnerability, above.

## Community Rank (optional)

Community Rank, only when `WHERETOKEN_COMMUNITY_URL` is set and participation is on, sends anonymous daily aggregates (UUID, local date, token total, optional API-equivalent estimated cost, client version). There is no public `WHERETOKEN_COMMUNITY_URL` — that is a remote deploy blocker. The optional cost field is never an actual bill; a missing price is omitted, not sent as $0. Community Rank is not a global, worldwide, or all-AI-users rank. It does not send prompts, sessions, paths, credentials, raw events, or the SQLite index. The rank service must not store connection IPs as user data.
