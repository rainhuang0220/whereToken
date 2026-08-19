# Community Rank

Community Rank is an **optional, anonymous, self-reported** comparison of
daily token totals among whereToken participants. It is **not** a global,
worldwide, or all-AI-users rank and **not** an audited leaderboard.

There is no public whereToken rank cluster. Nothing is uploaded unless you
set `WHERETOKEN_COMMUNITY_URL` (or run `wheretoken community serve` and
point at it).

## What crosses the network

Only an already-aggregated object:

- `participant_id` (random UUID, not a username)
- local calendar `period` (`YYYY-MM-DD`) and UTC offset
- `tokens` for that local day
- optional `estimated_cost_usd` + `cost_status` (omitted when unavailable)
- `client_version`

The client never sends usage events, prompts, transcripts, session or
request ids, file paths, workspaces, JWTs, cookies, API keys, hostnames,
or the SQLite index.

`WHERETOKEN_COMMUNITY_URL` must be `http` or `https` with a host and no
userinfo. `file:` and other schemes are ignored. The client does not follow
redirects, so an upload is not replayed onto a third-party hop.

Unknown cost is **not** rewritten as `$0`. A missing rank is **not**
printed as `#0`.

## Opt out

```text
wheretoken doctor
wheretoken community status
wheretoken community off
```

`--no-community` and `WHERETOKEN_COMMUNITY=off` skip upload and fetch for
that process (`DO_NOT_TRACK=1` does the same). `--offline` does not upload.

The local file is `~/.config/wheretoken/community.json` on Unix and
`%APPDATA%\whereToken\community.json` on Windows (override with
`WHERETOKEN_COMMUNITY_FILE`). It is not stored in the usage index.

## KPI

The fifth CLI / dashboard column is **估价** (row 1) and **排名** (row 2).
`--rank today` (default) or `--rank all` selects the standing. Rank is
competition ranking (ties share a place and skip). Below 20 participants
the product shows "not available yet" instead of a three-person podium.
**累计 / `--rank all` is the sum of days this client uploaded**, not the
kiln 全部 ledger. Miss a day of running whereToken and that day never
enters 累计.

## Identity

- `community.json` holds one random UUID. Reinstall that **keeps** the
  file is the same participant. Deleting the file mints a new UUID.
- `community off` then `on` keeps the same UUID; leave wipes remote days
  so 累计 starts empty.
- Copying `community.json` to another machine is one participant. Last
  write for a calendar day wins; the two ledgers are not summed.
- HMAC is not used in v1. The UUID is a bearer capability. That is
  enough for a self-hosted, self-reported board. It does not stop
  someone from inventing tokens.

## v1 threat decisions

| Threat | Impact | Likelihood | Mitigation | v1 |
| ------ | ------ | ---------- | ---------- | -- |
| Fake tokens | Rank lie | High if URL public | Cap, rate limit, self-reported copy | Accept |
| Fake UUIDs / Sybil | Unlock N≥20 podium | High if public | N≥20, IP limit | Accept |
| Replay PUT | Overwrite day | Medium | Last write wins | Intended |
| Duplicate honest scan | None | High | 5 min cache, last write | OK |
| Clock skew | ±2 UTC days stuffed | Low | periodNearNow | Accept |
| Multi-device same UUID | Last write per day | Medium | Document | Accept |
| Opt-out oracle | Learn if UUID joined | Low | Leave = never-seen | Mitigated |
| Rank scraping | See board | Low | No public URL | Accept |
| Privacy inference | Offset + version | Low | Aggregates only | Accept |
| HMAC | Would not stop self-lie | — | — | **Not added** |
