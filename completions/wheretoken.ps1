Register-ArgumentCompleter -Native -CommandName wheretoken -ScriptBlock {
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
    default { @('serve','scan','sources','doctor','rebuild','update','uninstall','completion','help','version','--help','--version','--json','--today','--since','--from','--to','--ascii','--no-color','--quiet','--offline','--tool','--vendor','--model','--claude','--kimi','--grok','--codex','--opencode','--cursor','--trae','--home','--port','--width') }
  }
  $cmds | Where-Object { $_ -like "$wordToComplete*" } | ForEach-Object {
    [System.Management.Automation.CompletionResult]::new($_, $_, 'ParameterName', $_)
  }
}
