package cli

const helpText = `wheretoken — local coding-agent token usage, as a character table.

USAGE
  wheretoken [flags]
  wheretoken serve [--port 8787]
  wheretoken scan [--json]
  wheretoken sources
  wheretoken completion bash|zsh|fish|powershell

INSTALL
  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest
  curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
  irm https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.ps1 | iex
  npm install -g wheretoken          # GitHub Release binary; no Go required
  npx wheretoken

With no flags, scans ledgers already on this machine and prints six figures
since records began: total (M), cache hit rate, max streak, current streak,
requests, user turns. Then a 7-day spark and a ranking of tools and vendors
(with share of total). Degraded Trae/Cursor logins become footnotes.

FLAGS
  --today              only today (local timezone; weeks start Monday)
  --tool NAME          slice by tool (claude, kimi, codex, opencode, cursor, trae)
  --vendor NAME        slice by vendor (anthropic, moonshot, minimax, …)
  --model NAME         slice by model id
  --claude --kimi --codex --opencode --cursor --trae
                       same as --tool=that-id
  --json               JSON on stdout (schema 1; tables stay the default)
  --ascii              ASCII box drawing (also auto on old Windows consoles)
  --no-color           no ANSI (NO_COLOR in the environment does the same)
  -q, --quiet          no scan-progress lines on stderr
  --offline            local ledgers only; skip Cursor/Trae account APIs
  --home DIR           fake home directory (tests)
  --width N            cap ranking table width (COLUMNS does the same)

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
  2  usage error (unknown command, tool, vendor, or model)

PRIVACY
  Read-only local files. No telemetry. Never prints JWTs or access tokens.
  serve binds 127.0.0.1 only. Do not paste secrets into issues.

EXAMPLES
  wheretoken
  wheretoken --today
  wheretoken --cursor
  wheretoken --today --cursor
  wheretoken --tool=claude --json
  wheretoken --offline --quiet
  wheretoken serve
  wheretoken completion zsh

Dashboard: wheretoken serve   →  http://127.0.0.1:8787
`

func HelpText() string { return helpText }
