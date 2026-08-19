# Community Rank

Community Rank is an **optional, anonymous, self-reported** comparison of
daily token totals among whereToken participants. It is **not** a global
developer ranking and **not** an audited leaderboard.

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

Unknown cost is **not** rewritten as `$0`. A missing rank is **not**
printed as `#0`.

## Opt out

```text
wheretoken community off
```

`--no-community` and `WHERETOKEN_COMMUNITY=off` skip upload and fetch for
that process. `--offline` does not upload.

The local file is `~/.config/wheretoken/community.json` (override with
`WHERETOKEN_COMMUNITY_FILE`). It is not stored in the usage index.

## KPI

The fifth CLI / dashboard column is **估价** (row 1) and **排名** (row 2).
`--rank today` (default) or `--rank all` selects the standing. Rank is
competition ranking (ties share a place and skip). Below 20 participants
the product shows "not available yet" instead of a three-person podium.
