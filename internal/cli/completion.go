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
  local opts="serve scan sources completion help version --help --version --json --today --ascii --no-color --quiet -q --offline --tool --vendor --model --claude --kimi --codex --opencode --cursor --trae --home --port --width"
  COMPREPLY=( $(compgen -W "${opts}" -- "${cur}") )
}
complete -F _wheretoken wheretoken
`

const zshCompletion = `#compdef wheretoken
_arguments -s \
  '(-h --help)'{-h,--help}'[help]' \
  '(-V --version)'{-V,--version}'[version]' \
  '--json[JSON on stdout]' \
  '--today[only today]' \
  '--ascii[ASCII box drawing]' \
  '--no-color[disable ANSI]' \
  '(-q --quiet)'{-q,--quiet}'[no progress on stderr]' \
  '--offline[skip Cursor/Trae account APIs]' \
  '--tool[tool id]:tool:(claude kimi codex opencode cursor trae)' \
  '--vendor[vendor id]:vendor:(anthropic moonshot openai minimax google deepseek doubao zhipu alibaba unknown)' \
  '--model[model id]:model:' \
  '--claude[slice Claude Code]' \
  '--kimi[slice Kimi Code]' \
  '--codex[slice Codex]' \
  '--opencode[slice OpenCode]' \
  '--cursor[slice Cursor]' \
  '--trae[slice Trae]' \
  '--home[fake home]:dir:_files -/' \
  '--port[serve port]:port:' \
  '--width[table width]:cols:' \
  '1:command:(serve scan sources completion help version)'
`

const fishCompletion = `complete -c wheretoken -f
complete -c wheretoken -n "__fish_use_subcommand" -a "serve scan sources completion help version"
complete -c wheretoken -l help -s h
complete -c wheretoken -l version -s V
complete -c wheretoken -l json
complete -c wheretoken -l today
complete -c wheretoken -l ascii
complete -c wheretoken -l no-color
complete -c wheretoken -l quiet -s q
complete -c wheretoken -l offline
complete -c wheretoken -l tool -r -a "claude kimi codex opencode cursor trae"
complete -c wheretoken -l vendor -r
complete -c wheretoken -l model -r
complete -c wheretoken -l claude -l kimi -l codex -l opencode -l cursor -l trae
complete -c wheretoken -l home -r
complete -c wheretoken -l port -r
complete -c wheretoken -l width -r
`

const powershellCompletion = `Register-ArgumentCompleter -Native -CommandName wheretoken -ScriptBlock {
  param($wordToComplete)
  $cmds = @('serve','scan','sources','completion','help','version','--help','--version','--json','--today','--ascii','--no-color','--quiet','--offline','--claude','--kimi','--codex','--opencode','--cursor','--trae','--width')
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
  }
}
`
