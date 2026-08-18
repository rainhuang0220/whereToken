Register-ArgumentCompleter -Native -CommandName wheretoken -ScriptBlock {
  param($wordToComplete, $commandAst)
  $cmd = ''
  foreach ($el in @($commandAst.CommandElements)) {
    $t = [string]$el
    switch ($t) {
      'serve' { $cmd = $t }
      'scan' { $cmd = $t }
      'sources' { $cmd = $t }
      'completion' { $cmd = $t }
      'help' { $cmd = $t }
      'version' { $cmd = $t }
    }
  }
  $cmds = switch ($cmd) {
    'scan' { @('--json','--quiet','--offline','--home','--ascii','--no-color','--help') }
    'serve' { @('--port','--offline','--quiet','--home','--help','--ascii','--no-color') }
    'sources' { @('--quiet','--offline','--home','--help','--ascii','--no-color') }
    'completion' { @('bash','zsh','fish','powershell','--quiet','--help') }
    default { @('serve','scan','sources','completion','help','version','--help','--version','--json','--today','--ascii','--no-color','--quiet','--offline','--tool','--vendor','--model','--claude','--kimi','--grok','--codex','--opencode','--cursor','--trae','--home','--port','--width') }
  }
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
  }
}
