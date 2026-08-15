Register-ArgumentCompleter -Native -CommandName wheretoken -ScriptBlock {
  param($wordToComplete)
  $cmds = @('serve','scan','sources','completion','help','version','--help','--version','--json','--today','--ascii','--no-color','--quiet','--claude','--kimi','--codex','--opencode','--cursor','--trae','--width')
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
  }
}
