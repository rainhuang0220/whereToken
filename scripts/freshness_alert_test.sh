#!/bin/bash
# Isolated freshness-alert test. A fake gh stands in for GitHub.
# Search-index lag returns no issues; the real list still dedups.
set -euo pipefail

ROOT="$(cd "$(dirname "$0")/.." && pwd)"
tmp="$(mktemp -d)"
trap 'rm -rf "$tmp"' EXIT
store="$tmp/issues"
: > "$store"
mkdir -p "$tmp/bin"

cat > "$tmp/bin/gh" << 'EOF'
#!/bin/bash
set -euo pipefail
store="${GH_FAKE_STORE:?}"
cmd="${1:-}"
sub="${2:-}"
shift 2 || true
if [ "$cmd" = "issue" ] && [ "$sub" = "list" ]; then
  search=0
  for arg in "$@"; do
    if [ "$arg" = "--search" ]; then
      search=1
    fi
  done
  if [ "$search" = "1" ]; then
    printf '%s\n' '[]'
    exit 0
  fi
  state_filter="open"
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--state" ]; then
      state_filter="$arg"
    fi
    prev="$arg"
  done
  python3 -c 'import json,sys
rows=[]
for line in open(sys.argv[1], encoding="utf-8"):
    line=line.rstrip("\n")
    if not line:
        continue
    number,state,title=line.split("\t",3)[:3]
    if state==sys.argv[2]:
        rows.append({"number":int(number),"title":title,"createdAt":"2026-11-09T01:30:58Z"})
print(json.dumps(rows))' "$store" "$state_filter"
  exit 0
fi
if [ "$cmd" = "issue" ] && [ "$sub" = "create" ]; then
  title=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--title" ]; then
      title="$arg"
    fi
    prev="$arg"
  done
  number="$(python3 -c 'import sys
n=0
for line in open(sys.argv[1], encoding="utf-8"):
    if line.strip():
        n+=1
print(n+1)' "$store")"
  body=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--body" ]; then
      body="$arg"
    fi
    prev="$arg"
  done
  classes="$(printf '%s\n' "$body" | python3 -c 'import sys
for line in sys.stdin:
    if line.startswith("classes:"):
        print(line.split(":",1)[1].strip())
        break')"
  printf '%s\topen\t%s\t%s\n' "$number" "$title" "$classes" >> "$store"
  printf 'https://example.test/issues/%s\n' "$number"
  exit 0
fi
if [ "$cmd" = "issue" ] && [ "$sub" = "close" ]; then
  number="$1"
  python3 -c 'import sys
path,number=sys.argv[1],sys.argv[2]
rows=[]
found=False
for line in open(path, encoding="utf-8"):
    line=line.rstrip("\n")
    if not line:
        continue
    n,state,title,classes=(line.split("\t",3)+[""])[:4]
    if n==number and state=="open":
        state="closed"
        found=True
    rows.append("%s\t%s\t%s\t%s" % (n,state,title,classes))
if not found:
    sys.exit(1)
open(path,"w",encoding="utf-8").write("\n".join(rows)+("\n" if rows else ""))' "$store" "$number"
  exit 0
fi
if [ "$cmd" = "issue" ] && [ "$sub" = "reopen" ]; then
  number="$1"
  python3 -c 'import sys
path,number=sys.argv[1:]
rows=[]
for line in open(path,encoding="utf-8"):
    n,state,title,classes=(line.rstrip("\n").split("\t",3)+[""])[:4]
    if n==number and state=="closed":
        state="open"
    rows.append("\t".join((n,state,title,classes)))
open(path,"w",encoding="utf-8").write("\n".join(rows)+"\n")' "$store" "$number"
  exit 0
fi
if [ "$cmd" = "issue" ] && [ "$sub" = "edit" ]; then
  number="$1"
  body=""
  prev=""
  for arg in "$@"; do
    if [ "$prev" = "--body" ]; then
      body="$arg"
    fi
    prev="$arg"
  done
  classes="$(printf '%s\n' "$body" | python3 -c 'import sys
for line in sys.stdin:
    if line.startswith("classes:"):
        print(line.split(":",1)[1].strip())
        break')"
  python3 -c 'import sys
path,number,classes=sys.argv[1:]
rows=[]
for line in open(path,encoding="utf-8"):
    n,state,title,old=(line.rstrip("\n").split("\t",3)+[""])[:4]
    rows.append("\t".join((n,state,title,classes if n==number else old)))
open(path,"w",encoding="utf-8").write("\n".join(rows)+"\n")' "$store" "$number" "$classes"
  exit 0
fi
if [ "$cmd" = "issue" ] && [ "$sub" = "view" ]; then
  number="$1"
  python3 -c 'import json,sys
path,number=sys.argv[1:]
body=""
for line in open(path,encoding="utf-8"):
    n,state,title,classes=(line.rstrip("\n").split("\t",3)+[""])[:4]
    if n==number:
        body="classes: %s\n" % classes
print(json.dumps({"body": body}))' "$store" "$number"
  exit 0
fi
if [ "$cmd" = "workflow" ] && [ "$sub" = "run" ]; then
  printf 'run\n' >> "${GH_FAKE_DISPATCH:?}"
  exit 0
fi
echo "unexpected gh $cmd $sub" >&2
exit 1
EOF
chmod +x "$tmp/bin/gh"

# Check on a later UTC day than legacy issue #42's creation time. A title
# migration must retain that original date rather than today's date.
cat > "$tmp/bin/date" << 'EOF'
#!/bin/bash
[ "$*" = '-u +%Y-%m-%d' ] || exit 1
printf '%s\n' '2026-11-10'
EOF
chmod +x "$tmp/bin/date"

export PATH="$tmp/bin:$PATH"
export GH_FAKE_STORE="$store"
export GH_FAKE_DISPATCH="$tmp/dispatch"
export GITHUB_REPOSITORY="example/whereToken"
export WHERETOKEN_DISPATCH_STAMP="$tmp/stamp"
: > "$tmp/dispatch"

out="$(bash "$ROOT/scripts/freshness-alert.sh" --surface actions --classes check_failed)"
printf '%s\n' "$out" | grep -q 'class=check_failed surface=actions opened'
grep -q $'\topen\tprofile-freshness alert (actions)\tcheck_failed$' "$store"

again="$(bash "$ROOT/scripts/freshness-alert.sh" --surface actions --classes 'wall_stale collection_stale')"
printf '%s\n' "$again" | grep -q 'issue=1 updated'
[ "$(grep -c $'\topen\t' "$store")" = "1" ]
grep -q $'\topen\tprofile-freshness alert (actions)\twall_stale collection_stale$' "$store"

local_out="$(bash "$ROOT/scripts/freshness-alert.sh" --surface local --classes publisher_stuck)"
printf '%s\n' "$local_out" | grep -q 'surface=local opened'
[ "$(grep -c $'\topen\t' "$store")" = "2" ]

pages_out="$(bash "$ROOT/scripts/freshness-alert.sh" --surface pages --classes pages_stale)"
printf '%s\n' "$pages_out" | grep -q 'surface=pages opened'
[ "$(grep -c $'\topen\t' "$store")" = "3" ]

recovered="$(bash "$ROOT/scripts/freshness-alert.sh" --surface actions --classes ok)"
printf '%s\n' "$recovered" | grep -q 'class=ok surface=actions'
if grep -q $'\topen\tprofile-freshness alert (actions)\t' "$store"; then
  echo "actions recovery closed the wrong incident or left its own open" >&2
  exit 1
fi
grep -q $'\topen\tprofile-freshness alert (local)\t' "$store"
grep -q $'\topen\tprofile-freshness alert (pages)\t' "$store"

same="$(bash "$ROOT/scripts/freshness-alert.sh" --surface local --classes publisher_stuck)"
printf '%s\n' "$same" | grep -q 'already-open'
[ "$(grep -c $'\topen\tprofile-freshness alert (local)\t' "$store")" = "1" ]

printf '42\topen\tprofile-freshness alert\n' >> "$store"
legacy="$(bash "$ROOT/scripts/freshness-alert.sh" --surface actions --classes ok)"
printf '%s\n' "$legacy" | grep -q 'class=ok surface=actions'
grep -q $'^42\topen\tprofile-freshness alert$' "$store"

silent="$(bash "$ROOT/scripts/freshness-alert.sh" --surface local --classes workflow_silent)"
printf '%s\n' "$silent" | grep -q 'dispatch=requested'
[ "$(wc -l < "$tmp/dispatch" | tr -d ' ')" = "1" ]
silent_again="$(bash "$ROOT/scripts/freshness-alert.sh" --surface local --classes workflow_silent)"
printf '%s\n' "$silent_again" | grep -q 'dispatch=skipped'
[ "$(wc -l < "$tmp/dispatch" | tr -d ' ')" = "1" ]

# A failed issue list must not look like recovery and must not open a ticket.
cat > "$tmp/bin/gh" << 'EOF'
#!/bin/bash
echo "" 
exit 0
EOF
chmod +x "$tmp/bin/gh"
set +e
bad="$(bash "$ROOT/scripts/freshness-alert.sh" --surface actions --classes ok 2>/dev/null)"
bad_code=$?
set -e
[ "$bad_code" -ne 0 ]
printf '%s\n' "$bad" | grep -q 'lookup-failed'
if printf '%s\n' "$bad" | grep -q 'class=ok'; then
  echo "lookup failure printed class=ok" >&2
  exit 1
fi

yml="$ROOT/.github/workflows/profile-freshness.yml"
grep -q "issues: write" "$yml"
grep -q "steps.wall.outcome == 'failure'" "$yml"
grep -q "steps.wall.outputs.class || 'check_failed'" "$yml"
grep -q 'steps.pages.outputs.class' "$yml"
grep -q 'GH_TOKEN: ${{ github.token }}' "$yml"
grep -q -- '--surface actions' "$yml"
grep -q -- '--surface pages' "$yml"
mirror_line="$(grep -n 'name: Verify and repair the deployed Pages fallback' "$yml" | cut -d: -f1)"
alert_line="$(grep -n 'name: Open or close one alert issue' "$yml" | cut -d: -f1)"
if ! [ "$mirror_line" -lt "$alert_line" ]; then
  echo "alert failure must not prevent Pages regeneration or repair" >&2
  exit 1
fi
if grep -E '^[[:space:]]*pull_request' "$yml"; then
  echo "profile-freshness must not run on pull_request" >&2
  exit 1
fi

echo "freshness alert isolated test ok"
