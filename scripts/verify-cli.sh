#!/usr/bin/env bash
# Run the CLI against the Kimi fixture home — never the developer $HOME.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
export WHERETOKEN_EXTRA_ROOTS=
dir="$(mktemp -d)"
trap 'rm -rf "$dir"' EXIT
dst="$dir/.kimi-code/sessions/x/s/agents/main"
mkdir -p "$dst"
cp testdata/adapters/kimi/session/agents/main/wire.jsonl "$dst/wire.jsonl"
go build -o "$dir/wheretoken" ./cmd/wheretoken

ver="$("$dir/wheretoken" --version)"
echo "$ver" | grep -q wheretoken

"$dir/wheretoken" --help | grep -q USAGE
"$dir/wheretoken" --help | grep -q 'EXIT CODES'
"$dir/wheretoken" --help | grep -q 'curl -fsSL'
"$dir/wheretoken" --help | grep -q 刷新
"$dir/wheretoken" --help | grep -q rebuild
"$dir/wheretoken" --help | grep -q update
"$dir/wheretoken" --help | grep -q uninstall
"$dir/wheretoken" --help | grep -q -- '--since'
"$dir/wheretoken" --help | grep -q -- '--rank'
"$dir/wheretoken" --help | grep -q -- '--no-community'
"$dir/wheretoken" --help | grep -q 'uploaded days'
"$dir/wheretoken" --help | grep -q community
if "$dir/wheretoken" --help | grep -q GOPATH; then
  echo "help lectures GOPATH" >&2
  exit 1
fi
if "$dir/wheretoken" --help | grep -q 'npm install'; then
  echo "help advertises unpublished npm" >&2
  exit 1
fi

out="$("$dir/wheretoken" --home "$dir" --ascii --quiet)"
echo "$out"
echo "$out" | grep -q '0.0012 M'
echo "$out" | grep -q 'Kimi'
echo "$out" | grep -q '占比'
echo "$out" | grep -q '估价'
# The bottom-right KPI is the usage portrait; community rank never prints in
# the default report (it lives in `wheretoken community` and --json).
echo "$out" | grep -q '用户画像'
if echo "$out" | grep -q '排名'; then
  echo "table still prints rank" >&2
  exit 1
fi
if echo "$out" | grep -q '社区排名暂不可用'; then
  echo "table still prints the community-rank note" >&2
  exit 1
fi
if echo "$out" | grep -q '#0'; then
  echo "table printed #0" >&2
  exit 1
fi
if echo "$out" | grep -E '\$0\.00' >/dev/null; then
  echo "table printed \$0.00" >&2
  exit 1
fi
if echo "$out" | grep -q eyJ; then
  echo "jwt leaked" >&2
  exit 1
fi

json="$("$dir/wheretoken" --home "$dir" --json --quiet)"
echo "$json" | grep -q '"schema": 1'
echo "$json" | grep -q '"total_m"'
echo "$json" | grep -q '"total":'
echo "$json" | grep -q '"max_streak_days"'
echo "$json" | grep -q '"current_streak_days"'
if echo "$json" | grep -q '"events"'; then
  echo "raw events in --json" >&2
  exit 1
fi
echo "$json" | grep -q '"community"'
if echo "$json" | grep -Eq '"rank":[[:space:]]*0'; then
  echo "json shipped rank 0" >&2
  exit 1
fi
if echo "$json" | grep -q '#0'; then
  echo "json printed #0" >&2
  exit 1
fi
if echo "$json" | grep -E '\$0\.00' >/dev/null; then
  echo "json printed \$0.00" >&2
  exit 1
fi

st="$("$dir/wheretoken" --home "$dir" --quiet community status)"
echo "$st" | grep -q 'community rank: off'
if echo "$st" | grep -q '#0'; then
  echo "community status printed #0" >&2
  exit 1
fi
if echo "$st" | grep -E '\$0\.00' >/dev/null; then
  echo "community status printed \$0.00" >&2
  exit 1
fi

nc="$("$dir/wheretoken" --home "$dir" --no-community --ascii --quiet)"
echo "$nc" | grep -q '用户画像'
if echo "$nc" | grep -q '排名'; then
  echo "--no-community still prints rank" >&2
  exit 1
fi
if echo "$nc" | grep -q '#0'; then
  echo "--no-community printed #0" >&2
  exit 1
fi
if echo "$nc" | grep -E '\$0\.00' >/dev/null; then
  echo "--no-community printed \$0.00" >&2
  exit 1
fi

doc="$("$dir/wheretoken" --home "$dir" --no-community --quiet doctor)"
echo "$doc" | grep -q 'Off this run'
echo "$doc" | grep -q 'DO_NOT_TRACK'
if echo "$doc" | grep -q '#0'; then
  echo "doctor --no-community printed #0" >&2
  exit 1
fi
if echo "$doc" | grep -E '\$0\.00' >/dev/null; then
  echo "doctor --no-community printed \$0.00" >&2
  exit 1
fi
dnt="$(DO_NOT_TRACK=1 "$dir/wheretoken" --home "$dir" --quiet doctor)"
echo "$dnt" | grep -q 'Off this run'

vendor="$("$dir/wheretoken" --home "$dir" --vendor=moonshot --ascii --quiet)"
echo "$vendor" | grep -q 'Moonshot'
echo "$vendor" | grep -q '0.0012 M'

since_json="$("$dir/wheretoken" --home "$dir" --since 7d --json --quiet)"
echo "$since_json" | grep -q '"schema": 1'
echo "$since_json" | grep -q '近 7 天'

"$dir/wheretoken" --home "$dir" rebuild --quiet >/dev/null

today_json="$("$dir/wheretoken" --home "$dir" --today --json --quiet)"
if echo "$today_json" | grep -q '"last_7d"'; then
  echo "today json leaked last_7d" >&2
  exit 1
fi

off="$("$dir/wheretoken" --home "$dir" --offline --ascii --quiet)"
# SpriteScene is three lines; the offline banner sits under it, before the KPI box.
echo "$off" | grep -q 'offline · 只用本机账本'
kpi="${off%%总用量*}"
if ! echo "$kpi" | grep -q 'offline ·'; then
  echo "offline banner should sit above the KPI table" >&2
  exit 1
fi

set +e
err="$("$dir/wheretoken" --home "$dir" --vendor=anthropc --quiet 2>&1)"
code=$?
set -e
test "$code" -eq 2
echo "$err" | grep -q anthropic

empty="$(mktemp -d)"
zeros="$("$dir/wheretoken" --home "$empty" --ascii --quiet)"
echo "$zeros" | grep -q '0.00 M'
echo "$zeros" | grep -q '本机没有找到账本'
echo "$zeros" | grep -q '估价'
echo "$zeros" | grep -q '用户画像'
if echo "$zeros" | grep -q '排名'; then
  echo "empty home printed rank" >&2
  exit 1
fi
if echo "$zeros" | grep -q '#0'; then
  echo "empty home printed #0" >&2
  exit 1
fi
if echo "$zeros" | grep -E '\$0\.00' >/dev/null; then
  echo "empty home printed \$0.00" >&2
  exit 1
fi
empty_json="$("$dir/wheretoken" --home "$empty" --json --quiet)"
echo "$empty_json" | grep -q '"max_streak_days": 0'
echo "$empty_json" | grep -q '"current_streak_days": 0'
empty_today="$("$dir/wheretoken" --home "$empty" --today --ascii --quiet)"
echo "$empty_today" | grep -q '本机没有找到账本'
if echo "$empty_today" | grep -q '今天还没有用量'; then
  echo "empty-home --today pretended a ledger exists" >&2
  exit 1
fi

scanjson="$("$dir/wheretoken" --home "$dir" --quiet scan)"
echo "$scanjson" | grep -q '"calendar"'
if echo "$scanjson" | grep -q '"schema": 1'; then
  echo "scan JSON must not pretend to be schema 1" >&2
  exit 1
fi
set +e
scan_today="$("$dir/wheretoken" --home "$dir" scan --today --quiet 2>&1)"
scan_today_code=$?
set -e
test "$scan_today_code" -eq 2
echo "$scan_today" | grep -q observatory

comp="$("$dir/wheretoken" completion zsh --quiet)"
echo "$comp" | grep -q '_arguments'

envhome="$(WHERETOKEN_HOME="$empty" "$dir/wheretoken" --ascii --quiet)"
echo "$envhome" | grep -q '0.00 M'
echo "$envhome" | grep -q '本机没有找到账本'
if echo "$envhome" | grep -q '占比'; then
  echo "WHERETOKEN_HOME ignored" >&2
  exit 1
fi

set +e
err="$("$dir/wheretoken" --home "$dir" --tool=windsurf --quiet 2>&1)"
code=$?
set -e
test "$code" -eq 2
echo "$err" | grep -q windsurf

set +e
err="$("$dir/wheretoken" --home "$dir" --tool=claud --quiet 2>&1)"
code=$?
set -e
test "$code" -eq 2
echo "$err" | grep -q claude

mod="$("$dir/wheretoken" --home "$dir" --model=k3 --ascii --quiet)"
echo "$mod" | grep -q '0.0012 M'
echo "$mod" | grep -q '用户回合'
modjson="$("$dir/wheretoken" --home "$dir" --model=k3 --json --quiet)"
echo "$modjson" | grep -q '"hide_turns": true'

today_kimi="$("$dir/wheretoken" --home "$dir" --today --kimi --ascii --quiet)"
echo "$today_kimi" | grep -q '今天'
echo "$today_kimi" | grep -q 'Kimi'
echo "$today_kimi" | grep -q '用户画像'
if echo "$today_kimi" | grep -q '排名'; then
  echo "--today still prints rank" >&2
  exit 1
fi
if echo "$today_kimi" | grep -q '0.0012 M'; then
  echo "--today --kimi mixed all-time rows" >&2
  exit 1
fi
echo "$today_kimi" | grep -q '0.00 M'
echo "$today_kimi" | grep -q '今天还没有用量'

set +e
err="$("$dir/wheretoken" --home "$dir" --nope --quiet 2>&1)"
code=$?
set -e
test "$code" -eq 2
echo "$err" | grep -q 'unknown flag'
if echo "$err" | grep -q 'flag provided but not defined'; then
  echo "unknown flag still uses Go jargon" >&2
  exit 1
fi

narrow="$(COLUMNS=40 "$dir/wheretoken" --home "$dir" --ascii --quiet)"
echo "$narrow" | grep -q 'Kimi'
if echo "$narrow" | grep -q '排名'; then
  echo "narrow output still prints rank" >&2
  exit 1
fi
narrow_flag="$("$dir/wheretoken" --home "$dir" --width 40 --ascii --quiet)"
echo "$narrow_flag" | grep -q 'Kimi'
if echo "$narrow" | grep -q 'K\.\.\.'; then
  echo "COLUMNS=40 destroyed the tool name" >&2
  exit 1
fi

src_err="$("$dir/wheretoken" sources --home "$empty" --quiet 2>&1 >/dev/null)"
echo "$src_err" | grep -q '没有找到本机账本'
src_err2="$("$dir/wheretoken" --home "$empty" --quiet sources 2>&1 >/dev/null)"
echo "$src_err2" | grep -q '没有找到本机账本'

echo "ok fixture CLI"
