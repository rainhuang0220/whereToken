package cli

const helpText = `wheretoken — local coding-agent token usage, as a character table.

USAGE
  wheretoken [flags]
  wheretoken [flags] serve [--port 8787]
  wheretoken [flags] scan          observatory JSON (not schema 1; no --today/--tool)
  wheretoken [flags] sources
  wheretoken [flags] doctor     sources plus Community Rank (no upload)
  wheretoken [flags] rebuild     wipe the local scan index and rescan
  wheretoken [flags] update      replace this binary with the latest GitHub Release
  wheretoken [flags] uninstall   remove this binary
  wheretoken [flags] community [status|on|off|serve]
  wheretoken [flags] completion bash|zsh|fish|powershell

INSTALL
  curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
  irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
  curl.exe -fsSL -o %TEMP%\wt-install.cmd https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.cmd && %TEMP%\wt-install.cmd
  brew tap rainhuang0220/wheretoken && brew install wheretoken
  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
  brew install --HEAD ./Formula/wheretoken.rb  # from a clone
No npm package. GitHub Release binaries are unsigned.
go install and brew --HEAD embed the CLI table; the kiln dashboard is in
GitHub Release and brew tap rainhuang0220/wheretoken.

With no flags, scans ledgers already on this machine and prints the KPI box
since records began: total (M), cache hit rate, streaks, today/peak,
requests, user turns, estimated cost, and community rank. Then a 7-day spark
and a ranking of tools and vendors (with share of total). Degraded Trae/Cursor
logins become footnotes. Community rank is self-reported anonymous aggregate
usage among participants, not a global developer leaderboard.

FLAGS
  -h, --help           this text
  -V, --version        print version
  --today              only today in the local timezone
  --since 7d           last N local calendar days including today (7d, 30d)
  --from DATE          inclusive start (YYYY-MM-DD or RFC3339, local timezone)
  --to DATE            inclusive end date (YYYY-MM-DD or RFC3339, local timezone)
  --tool NAME          slice by tool (claude, kimi, grok, minimax, openclaw, codex, opencode, cursor, trae)
  --vendor NAME        slice by vendor (anthropic, moonshot, minimax, xai, …)
  --model NAME         slice by model id (user turns are per-tool, so that KPI is —)
  --claude --kimi --grok --minimax --openclaw --codex --opencode --cursor --trae
                       same as --tool=that-id
  --json               JSON on stdout (schema 1; docs/cli-json.schema.json). scan is a different dump
  --ascii              ASCII box drawing (also auto on old Windows consoles)
  --no-color           no ANSI (NO_COLOR in the environment does the same)
  -q, --quiet          no scan-progress lines on stderr
  --offline            local ledgers only; skip Cursor/Trae APIs and community upload
  --rank today|all     community rank period in the fifth KPI column (default today)
  --no-community       do not upload or fetch community rank
  --home DIR           fake home directory (tests)
  --port N             serve bind port (default 8787; tries 8787–8797 if busy)
  --width N            cap ranking width; drop 估价 then 回合/请求 before truncating names

ENV
  NO_COLOR             disable ANSI (same as --no-color)
  WHERETOKEN_ASCII=1   ASCII box drawing
  WHERETOKEN_HOME      override home (same idea as --home)
  WHERETOKEN_OFFLINE=1 same as --offline
  WHERETOKEN_COMMUNITY=off  same as --no-community
  WHERETOKEN_COMMUNITY_URL  rank service (unset = no upload; no public cluster)
  WHERETOKEN_COMMUNITY_FILE override community.json (Unix ~/.config/wheretoken, Windows %APPDATA%\\whereToken)
  WHERETOKEN_INDEX     path to the local scan cache (default ~/.cache/wheretoken/index.v1.db)
  WHERETOKEN_NO_INDEX=1  skip the scan cache
  WHERETOKEN_EXTRA_ROOTS   extra homes (Unix :, Windows ;, or commas)
  NO_UTF8              ASCII box drawing
  TERM=dumb            disable color
  FORCE_COLOR          ANSI even when stdout is not a TTY (NO_COLOR still wins)
  COLUMNS              cap ranking table width (same as --width)

EXIT CODES
  0  ok (including zero data, or a degraded Trae/Cursor login)
  1  runtime failure
  2  usage error (unknown command, tool, vendor, model, or flag)

PRIVACY
  Read-only local files. No telemetry. Never prints JWTs or access tokens.
  serve binds 127.0.0.1 only. Do not paste secrets into issues.
  Community Rank (when WHERETOKEN_COMMUNITY_URL is set and not opted out)
  uploads anonymous daily token totals only — never events, prompts, paths,
  credentials, or the SQLite index. It is not a global developer rank.
  wheretoken community off opts out. See docs/community.md.

EXAMPLES
  wheretoken
  wheretoken --today
  wheretoken --since 7d
  wheretoken --from 2026-08-01 --to 2026-08-19
  wheretoken rebuild
  wheretoken update
  wheretoken uninstall
  wheretoken --cursor
  wheretoken --today --cursor
  wheretoken --today --kimi
  wheretoken --today --grok
  wheretoken --tool=claude --json
  wheretoken --model=k3
  wheretoken sources
  wheretoken doctor
  wheretoken --offline --quiet
  wheretoken serve
  wheretoken community off
  wheretoken community serve
  wheretoken completion zsh

Dashboard: wheretoken serve   →  http://127.0.0.1:8787
页内「刷新」重新扫描本机；浏览器重载只会显示上次结果。
`

func HelpText() string { return helpText }
