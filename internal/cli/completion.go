package cli

import (
	"fmt"
	"strings"
)

func Completion(shell string) (string, error) {
	switch strings.ToLower(strings.TrimSpace(shell)) {
	case "bash":
		return bashCompletion, nil
	case "zsh":
		return zshCompletion, nil
	case "fish":
		return fishCompletion, nil
	case "powershell", "pwsh":
		return powershellCompletion, nil
	case "":
		return "", usageError{msg: "completion requires a shell: bash, zsh, fish, powershell"}
	default:
		return "", usageError{msg: fmt.Sprintf("unknown shell %q (bash, zsh, fish, powershell)", shell)}
	}
}

const bashCompletion = `# bash completion for wheretoken
_wheretoken() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local cmd="" i
  for ((i=1; i<COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      serve|scan|sources|doctor|rebuild|update|upgrade|uninstall|completion|help|version) cmd="${COMP_WORDS[i]}" ;;
    esac
  done
  local opts
  case "$cmd" in
    scan) opts="--json --quiet -q --offline --home --ascii --no-color --help" ;;
    serve) opts="--port --offline --quiet -q --home --help --ascii --no-color" ;;
    sources) opts="--quiet -q --offline --home --help --ascii --no-color" ;;
    doctor) opts="--quiet -q --offline --home --help --ascii --no-color" ;;
    rebuild) opts="--json --today --since --from --to --ascii --no-color --quiet -q --offline --tool --vendor --model --claude --kimi --grok --minimax --codex --opencode --cursor --trae --home --width --help" ;;
    update|upgrade) opts="--quiet -q --help" ;;
    uninstall) opts="--quiet -q --help" ;;
    completion) opts="bash zsh fish powershell --quiet -q --help" ;;
    *) opts="serve scan sources doctor rebuild update uninstall completion help version --help --version --json --today --since --from --to --ascii --no-color --quiet -q --offline --tool --vendor --model --claude --kimi --grok --minimax --codex --opencode --cursor --trae --home --port --width" ;;
  esac
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _wheretoken wheretoken
`

const zshCompletion = `#compdef wheretoken
_wheretoken() {
  local cmd w
  for w in $words; do
    case $w in
      serve|scan|sources|doctor|rebuild|update|upgrade|uninstall|completion|help|version) cmd=$w ;;
    esac
  done
  case $cmd in
    scan)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--json[observatory JSON]' \
        '--ascii[ASCII box drawing]' \
        '--no-color[disable ANSI]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--home[fake home]:dir:_files -/'
      ;;
    serve)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--home[fake home]:dir:_files -/' \
        '--port[serve port]:port:'
      ;;
    sources)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--home[fake home]:dir:_files -/'
      ;;
    doctor)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--home[fake home]:dir:_files -/'
      ;;
    rebuild)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--json[JSON on stdout]' \
        '--today[only today]' \
        '--since[last N days]:since:(7d 30d)' \
        '--from[start date]:from:' \
        '--to[end date]:to:' \
        '--ascii[ASCII box drawing]' \
        '--no-color[disable ANSI]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--tool[tool id]:tool:(claude kimi grok minimax codex opencode cursor trae)' \
        '--home[fake home]:dir:_files -/'
      ;;
    update|upgrade|uninstall)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]'
      ;;
    completion)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '1:shell:(bash zsh fish powershell)'
      ;;
    *)
      _arguments -s \
        '(-h --help)'{-h,--help}'[help]' \
        '(-V --version)'{-V,--version}'[version]' \
        '--json[JSON on stdout]' \
        '--today[only today]' \
        '--since[last N days]:since:(7d 30d)' \
        '--from[start date]:from:' \
        '--to[end date]:to:' \
        '--ascii[ASCII box drawing]' \
        '--no-color[disable ANSI]' \
        '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
        '--offline[skip Cursor/Trae account APIs]' \
        '--tool[tool id]:tool:(claude kimi grok minimax codex opencode cursor trae)' \
        '--vendor[vendor id]:vendor:(anthropic moonshot openai minimax google deepseek doubao zhipu alibaba xai unknown)' \
        '--model[model id]:model:' \
        '--claude[slice Claude Code]' \
        '--kimi[slice Kimi Code]' \
        '--grok[slice Grok]' \
        '--minimax[slice MiniMax Agent]' \
        '--codex[slice Codex]' \
        '--opencode[slice OpenCode]' \
        '--cursor[slice Cursor]' \
        '--trae[slice Trae]' \
        '--home[fake home]:dir:_files -/' \
        '--port[serve port]:port:' \
        '--width[table width]:cols:' \
        '1:command:(serve scan sources doctor rebuild update uninstall completion help version)'
      ;;
  esac
}
_wheretoken "$@"
`

const fishCompletion = `complete -c wheretoken -f
complete -c wheretoken -n "__fish_use_subcommand" -a "serve scan sources doctor rebuild update uninstall completion help version"
complete -c wheretoken -l help -s h
complete -c wheretoken -l version -s V
complete -c wheretoken -l json
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l today
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l since -r -a "7d 30d"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l from -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l to -r
complete -c wheretoken -l ascii
complete -c wheretoken -l no-color
complete -c wheretoken -l quiet -s q
complete -c wheretoken -l offline
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l tool -r -a "claude kimi grok minimax codex opencode cursor trae"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l vendor -r -a "anthropic moonshot openai minimax google deepseek doubao zhipu alibaba xai unknown"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l model -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l claude -l kimi -l grok -l minimax -l codex -l opencode -l cursor -l trae
complete -c wheretoken -l home -r -F
complete -c wheretoken -n "not __fish_seen_subcommand_from scan sources doctor completion" -l port -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion" -l width -r
complete -c wheretoken -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName wheretoken -ScriptBlock {
  param($wordToComplete, $commandAst)
  $cmd = ''
  foreach ($el in @($commandAst.CommandElements)) {
    $t = [string]$el
    switch ($t) {
      'serve' { $cmd = $t }
      'scan' { $cmd = $t }
      'sources' { $cmd = $t }
      'doctor' { $cmd = $t }
      'rebuild' { $cmd = $t }
      'update' { $cmd = $t }
      'upgrade' { $cmd = $t }
      'uninstall' { $cmd = $t }
      'completion' { $cmd = $t }
      'help' { $cmd = $t }
      'version' { $cmd = $t }
    }
  }
  $cmds = switch ($cmd) {
    'scan' { @('--json','--quiet','--offline','--home','--ascii','--no-color','--help') }
    'serve' { @('--port','--offline','--quiet','--home','--help','--ascii','--no-color') }
    'sources' { @('--quiet','--offline','--home','--help','--ascii','--no-color') }
    'doctor' { @('--quiet','--offline','--home','--help','--ascii','--no-color') }
    'rebuild' { @('--json','--today','--since','--from','--to','--ascii','--no-color','--quiet','--offline','--tool','--vendor','--model','--home','--width','--help') }
    'update' { @('--quiet','--help') }
    'upgrade' { @('--quiet','--help') }
    'uninstall' { @('--quiet','--help') }
    'completion' { @('bash','zsh','fish','powershell','--quiet','--help') }
    default { @('serve','scan','sources','doctor','rebuild','update','uninstall','completion','help','version','--help','--version','--json','--today','--since','--from','--to','--ascii','--no-color','--quiet','--offline','--tool','--vendor','--model','--claude','--kimi','--grok','--minimax','--codex','--opencode','--cursor','--trae','--home','--port','--width') }
  }
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
  }
}
`
