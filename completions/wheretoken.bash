# bash completion for wheretoken
_wheretoken() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local cmd="" i
  for ((i=1; i<COMP_CWORD; i++)); do
    case "${COMP_WORDS[i]}" in
      serve|scan|sources|completion|help|version) cmd="${COMP_WORDS[i]}" ;;
    esac
  done
  local opts
  case "$cmd" in
    scan) opts="--json --quiet -q --offline --home --ascii --no-color --help" ;;
    serve) opts="--port --offline --quiet -q --home --help --ascii --no-color" ;;
    sources) opts="--quiet -q --offline --home --help --ascii --no-color" ;;
    completion) opts="bash zsh fish powershell --quiet -q --help" ;;
    *) opts="serve scan sources completion help version --help --version --json --today --ascii --no-color --quiet -q --offline --tool --vendor --model --claude --kimi --grok --codex --opencode --cursor --trae --home --port --width" ;;
  esac
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _wheretoken wheretoken
