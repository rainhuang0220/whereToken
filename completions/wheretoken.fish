complete -c wheretoken -f
complete -c wheretoken -n "__fish_use_subcommand" -a "serve scan sources doctor rebuild update uninstall community pricing login logout sync completion card help version"
complete -c wheretoken -l help -s h
complete -c wheretoken -l version -s V
complete -c wheretoken -l json
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor pricing completion card" -l today
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor pricing completion card" -l since -r -a "7d 30d"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor pricing completion card" -l from -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor pricing completion card" -l to -r
complete -c wheretoken -l ascii
complete -c wheretoken -l no-color
complete -c wheretoken -l quiet -s q
complete -c wheretoken -l offline
complete -c wheretoken -n "__fish_seen_subcommand_from login" -l no-sync
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor pricing completion card" -l tool -r -a "claude kimi grok minimax openclaw opencode codex cursor trae gemini qwen cline roo kilo zcode"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion card" -l vendor -r -a "anthropic moonshot openai minimax google deepseek doubao zhipu alibaba xai unknown"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion card" -l model -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion card" -l claude -l kimi -l grok -l minimax -l openclaw -l codex -l opencode -l cursor -l trae
complete -c wheretoken -l home -r -F
complete -c wheretoken -n "not __fish_seen_subcommand_from scan sources doctor pricing completion card" -l port -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor completion card" -l width -r
complete -c wheretoken -n "not __fish_seen_subcommand_from scan serve sources doctor community pricing completion card" -l rank -r -a "today all"
complete -c wheretoken -n "not __fish_seen_subcommand_from scan sources community pricing completion card" -l no-community
complete -c wheretoken -n "__fish_seen_subcommand_from community" -a "status on off serve"
complete -c wheretoken -n "__fish_seen_subcommand_from completion" -a "bash zsh fish powershell"
complete -c wheretoken -n "__fish_seen_subcommand_from pricing" -l usage
complete -c wheretoken -n "__fish_seen_subcommand_from card" -k -a "(__fish_complete_suffix svg)"
