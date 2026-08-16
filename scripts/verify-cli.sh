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
if echo "$out" | grep -q eyJ; then
  echo "jwt leaked" >&2
  exit 1
fi

json="$("$dir/wheretoken" --home "$dir" --json --quiet)"
echo "$json" | grep -q '"schema": 1'
echo "$json" | grep -q '"total_m"'
echo "$json" | grep -q '"total":'
if echo "$json" | grep -q '"events"'; then
  echo "raw events in --json" >&2
  exit 1
fi

vendor="$("$dir/wheretoken" --home "$dir" --vendor=moonshot --ascii --quiet)"
echo "$vendor" | grep -q 'Moonshot'
echo "$vendor" | grep -q '0.0012 M'

today_json="$("$dir/wheretoken" --home "$dir" --today --json --quiet)"
if echo "$today_json" | grep -q '"last_7d"'; then
  echo "today json leaked last_7d" >&2
  exit 1
fi

off="$("$dir/wheretoken" --home "$dir" --offline --ascii --quiet)"
echo "$off" | head -n 2 | grep -q 'offline'

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
