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
  npm install -g wheretoken          # GitHub Release binary; no Go required
  npx wheretoken

With no flags, scans ledgers already on this machine and prints six figures
since records began: total (M), cache hit rate, max streak, current streak,
requests, user turns. Then a ranking of tools and vendors.

FLAGS
  --today              only today (local timezone; weeks start Monday)
  --tool NAME          slice by tool (claude, kimi, codex, opencode, cursor, trae)
  --vendor NAME        slice by vendor (anthropic, moonshot, minimax, …)
  --model NAME         slice by model id
  --claude --kimi --codex --opencode --cursor --trae
                       same as --tool=that-id
  --json               JSON on stdout (tables are the default)
  --ascii              ASCII box drawing (also auto on old Windows consoles)
  --no-color           no ANSI (NO_COLOR in the environment does the same)
  --home DIR           fake home directory (tests)
  -h, --help
  -V, --version

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
  wheretoken --tool=claude --json
  wheretoken serve

Dashboard: wheretoken serve   →  http://127.0.0.1:8787
`

func HelpText() string { return helpText }
