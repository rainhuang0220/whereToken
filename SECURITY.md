# Security

whereToken reads **local** coding-agent ledgers on the machine that runs it. It does not send telemetry. The dashboard does not load Google Fonts or other third-party assets. `wheretoken serve` binds **127.0.0.1** only and sets `X-Content-Type-Options: nosniff`, `X-Frame-Options: DENY`, and `Referrer-Policy: no-referrer`. `/api/scan` and `/api/summary` refuse a non-localhost `Host` (DNS rebinding).

The CLI never prints JWTs, access tokens, API keys, `Authorization`, `Cookie`, or `Set-Cookie` values. `wheretoken scan --json` and `/api/scan` run the same redaction. If a source is degraded (Trae encrypted login, Cursor signed out), it says so in a footnote instead of dumping secrets.

Do not paste `~/.config`, `%APPDATA%`, or session files into issues. Report vulnerabilities privately to the GitHub owner.
