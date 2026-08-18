complete -c wheretoken -f
complete -c wheretoken -n "__fish_use_subcommand" -a "serve scan sources completion help version"
complete -c wheretoken -l help -s h
complete -c wheretoken -l version -s V
complete -c wheretoken -l json
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l today
complete -c wheretoken -l ascii
complete -c wheretoken -l no-color
complete -c wheretoken -l quiet -s q
complete -c wheretoken -l offline
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l tool -r -a "claude kimi grok codex opencode cursor trae"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l vendor -r -a "anthropic moonshot openai minimax google deepseek doubao zhipu alibaba xai unknown"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l model -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l claude -l kimi -l grok -l codex -l opencode -l cursor -l trae
complete -c wheretoken -l home -r -F
complete -c wheretoken -n "not __fish_seen_subcommand_from scan sources completion" -l port -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources completion" -l width -r
complete -c wheretoken -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"
