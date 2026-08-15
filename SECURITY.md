# Security

whereToken reads **local** coding-agent ledgers on the machine that runs it. It does not send telemetry. `wheretoken serve` binds **127.0.0.1** only.

The CLI never prints JWTs, access tokens, API keys, or `Authorization` headers. If a source is degraded (Trae encrypted login, Cursor signed out), it says so in a footnote instead of dumping secrets.

Do not paste `~/.config`, `%APPDATA%`, or session files into issues. Report vulnerabilities privately to the GitHub owner.
