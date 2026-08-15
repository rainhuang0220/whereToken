#!/usr/bin/env bash
# Run the CLI against the Kimi fixture home — never the developer $HOME.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
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

echo "ok fixture CLI"
