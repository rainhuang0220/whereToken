# bash completion for wheretoken
_wheretoken() {
  local cur="${COMP_WORDS[COMP_CWORD]}"
  local opts="serve scan sources completion help version --help --version --json --today --ascii --no-color --quiet -q --tool --vendor --model --claude --kimi --codex --opencode --cursor --trae --home --port"
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _wheretoken wheretoken
