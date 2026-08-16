package cli

const helpText = `wheretoken — local coding-agent token usage, as a character table.

USAGE
  wheretoken [flags]
  wheretoken [flags] serve [--port 8787]
  wheretoken [flags] scan
  wheretoken [flags] sources
  wheretoken [flags] completion bash|zsh|fish|powershell

INSTALL
  curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
  irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
  brew tap rainhuang0220/wheretoken && brew install wheretoken
  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
  brew install --HEAD ./Formula/wheretoken.rb  # from a clone
No npm package. GitHub Release binaries are unsigned.
go install embeds the CLI table; the kiln dashboard is in GitHub Release / brew.

With no flags, scans ledgers already on this machine and prints six figures
since records began: total (M), cache hit rate, max streak, current streak,
requests, user turns. Then a 7-day spark and a ranking of tools and vendors
(with share of total). Degraded Trae/Cursor logins become footnotes.

FLAGS
  -h, --help           this text
  -V, --version        print version
  --today              only today in the local timezone
  --tool NAME          slice by tool (claude, kimi, codex, opencode, cursor, trae)
  --vendor NAME        slice by vendor (anthropic, moonshot, minimax, xai, …)
  --model NAME         slice by model id (user turns are per-tool, so that KPI is —)
  --claude --kimi --codex --opencode --cursor --trae
                       same as --tool=that-id
  --json               JSON on stdout (schema 1; docs/cli-json.schema.json)
  --ascii              ASCII box drawing (also auto on old Windows consoles)
  --no-color           no ANSI (NO_COLOR in the environment does the same)
  -q, --quiet          no scan-progress lines on stderr
  --offline            local ledgers only; skip Cursor/Trae APIs (table and serve)
  --home DIR           fake home directory (tests)
  --port N             serve bind port (default 8787; tries 8787–8797 if busy)
  --width N            cap ranking width; drop 回合/请求 before truncating names

ENV
  NO_COLOR             disable ANSI (same as --no-color)
  WHERETOKEN_ASCII=1   ASCII box drawing
  WHERETOKEN_HOME      override home (same idea as --home)
  WHERETOKEN_OFFLINE=1 same as --offline
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

EXAMPLES
  wheretoken
  wheretoken --today
  wheretoken --cursor
  wheretoken --today --cursor
  wheretoken --today --kimi
  wheretoken --tool=claude --json
  wheretoken --model=k3
  wheretoken sources
  wheretoken --offline --quiet
  wheretoken serve
  wheretoken completion zsh

Dashboard: wheretoken serve   →  http://127.0.0.1:8787
页内「刷新」重新扫描本机；浏览器重载只会显示上次结果。
`

func HelpText() string { return helpText }
