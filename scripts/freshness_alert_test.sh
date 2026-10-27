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
  python3 -c 'import json,sys
rows=[]
for line in open(sys.argv[1], encoding="utf-8"):
    line=line.rstrip("\n")
    if not line:
        continue
    number,state,title=line.split("\t",2)
    if state=="open":
        rows.append({"number":int(number),"title":title})
print(json.dumps(rows))' "$store"
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
  printf '%s\topen\t%s\n' "$number" "$title" >> "$store"
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
    n,state,title=line.split("\t",2)
    if n==number and state=="open":
        state="closed"
        found=True
    rows.append("%s\t%s\t%s" % (n,state,title))
if not found:
    sys.exit(1)
open(path,"w",encoding="utf-8").write("\n".join(rows)+("\n" if rows else ""))' "$store" "$number"
  exit 0
fi
echo "unexpected gh $cmd $sub" >&2
exit 1
EOF
chmod +x "$tmp/bin/gh"

export PATH="$tmp/bin:$PATH"
export GH_FAKE_STORE="$store"
export GITHUB_REPOSITORY="example/whereToken"

out="$(bash "$ROOT/scripts/freshness-alert.sh" --class check_failed)"
printf '%s\n' "$out" | grep -q 'class=check_failed opened'
count="$(grep -c $'\topen\t' "$store" || true)"
[ "$count" = "1" ]
grep -q $'\topen\tprofile-freshness alert$' "$store"

again="$(bash "$ROOT/scripts/freshness-alert.sh" --class wall_stale)"
printf '%s\n' "$again" | grep -q 'already-open'
count="$(grep -c $'\topen\t' "$store" || true)"
[ "$count" = "1" ]

recovered="$(bash "$ROOT/scripts/freshness-alert.sh" --class ok)"
printf '%s\n' "$recovered" | grep -q 'class=ok'
if grep -q $'\topen\t' "$store"; then
  echo "recovery left an open alert" >&2
  exit 1
fi
grep -q $'\tclosed\tprofile-freshness alert$' "$store"

yml="$ROOT/.github/workflows/profile-freshness.yml"
grep -q "issues: write" "$yml"
grep -q "steps.wall.outcome == 'failure'" "$yml"
grep -q "steps.wall.outputs.class || 'check_failed'" "$yml"
grep -q 'GH_TOKEN: ${{ github.token }}' "$yml"
if grep -E '^[[:space:]]*pull_request' "$yml"; then
  echo "profile-freshness must not run on pull_request" >&2
  exit 1
fi

echo "freshness alert isolated test ok"
