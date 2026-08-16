complete -c wheretoken -f
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
complete -c wheretoken -l vendor -r -a "anthropic moonshot openai minimax google deepseek doubao zhipu alibaba xai unknown"
complete -c wheretoken -l model -r
complete -c wheretoken -l claude -l kimi -l codex -l opencode -l cursor -l trae
complete -c wheretoken -l home -r
complete -c wheretoken -l port -r
complete -c wheretoken -l width -r
