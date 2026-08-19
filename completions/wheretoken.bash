# bash completion for wheretoken
_wheretoken() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local cmd="" i
  for ((i=1; i<COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      serve|scan|sources|doctor|rebuild|update|upgrade|uninstall|community|completion|help|version) cmd="${COMP_WORDS[i]}" ;;
    esac
  done
  local opts
  case "$cmd" in
    scan) opts="--json --quiet -q --offline --home --ascii --no-color --help" ;;
    serve) opts="--port --offline --quiet -q --home --help --ascii --no-color" ;;
    sources) opts="--quiet -q --offline --home --help --ascii --no-color" ;;
    doctor) opts="--quiet -q --offline --home --help --ascii --no-color" ;;
    rebuild) opts="--json --today --since --from --to --ascii --no-color --quiet -q --offline --rank --no-community --tool --vendor --model --claude --kimi --grok --minimax --openclaw --codex --opencode --cursor --trae --home --width --help" ;;
    update|upgrade) opts="--quiet -q --help" ;;
    uninstall) opts="--quiet -q --help" ;;
    community) opts="status on off serve --port --offline --quiet -q --home --help" ;;
    completion) opts="bash zsh fish powershell --quiet -q --help" ;;
    *) opts="serve scan sources doctor rebuild update uninstall community completion help version --help --version --json --today --since --from --to --ascii --no-color --quiet -q --offline --rank --no-community --tool --vendor --model --claude --kimi --grok --minimax --openclaw --codex --opencode --cursor --trae --home --port --width" ;;
  esac
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _wheretoken wheretoken
