#!/bin/bash
# Classify the public wall and open or close one GitHub issue.
# A fresh result closes the open alert. A bad result opens one issue
# and does not comment again while that issue stays open.
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-rainhuang0220/whereToken}"
# One open issue per surface. A date or a shared prefix would let one
# recovery close a different surface, or keep the first reason forever.
TITLE_PREFIX="profile-freshness alert"
WALL_URL="${WHERETOKEN_WALL_URL:-https://wheretoken.plainlist.space/api/v1/public-profile/rainhuang0220}"
STATE_FILE="${WHERETOKEN_REFRESH_STATE:-${HOME}/.config/wheretoken/profile-refresh-state.json}"
HERE="$(cd "$(dirname "$0")" && pwd)"

usage() {
  echo "usage: freshness-alert.sh [--classify-only | --surface SURFACE --classes CLASSES | --drill]" >&2
  exit 2
}

surface_title() {
  case "$1" in
    actions|local|pages) printf '%s (%s)\n' "$TITLE_PREFIX" "$1" ;;
    *) usage ;;
  esac
}

body_for() {
  case "$1" in
    wall_stale)
      cat <<'EOF'
class: wall_stale

The public wall's verified_at is older than 36 hours.
This check sees only the public profile. It cannot tell whether the Mac was offline, the publisher skipped, or the wall itself stopped moving.
This is not a new token measurement.
EOF
      ;;
    collection_stale)
      cat <<'EOF'
class: collection_stale

The hosted projection or local refresh reports incomplete source coverage.
Check `wheretoken profile refresh doctor` for the affected sources, UTC gaps and recovery steps.
EOF
      ;;
    check_failed)
      cat <<'EOF'
class: check_failed

The freshness check could not read the public wall.
This check sees only the public profile. It cannot tell whether the Mac was offline.
This is not a new token measurement.
EOF
      ;;
    mac_offline)
      cat <<'EOF'
class: mac_offline

The public wall is older than 48 hours and this Mac has no publisher check in the last 12 hours.
The wall was not marked stale just because Actions missed a schedule.
This is not a new token measurement.
EOF
      ;;
    publisher_stuck)
      cat <<'EOF'
class: publisher_stuck

This Mac checked recently, and the public wall is still older than 48 hours.
This is not a new token measurement.
EOF
      ;;
    workflow_silent)
      cat <<'EOF'
class: workflow_silent

The public wall is still inside the local freshness window, and the profile-freshness workflow has no schedule or dispatch in the last 6 hours.
GitHub Actions did not report this gap itself.
This is not a new token measurement.
EOF
      ;;
    workflow_failed)
      cat <<'EOF'
class: workflow_failed

The latest profile-freshness schedule or dispatch in the last 6 hours completed with a failure conclusion.
A failed query of the Actions list is not this class.
This is not a new token measurement.
EOF
      ;;
    *)
      echo "class: $1"
      echo
      echo "This is not a new token measurement."
      ;;
  esac
}

# Exact title only. Legacy "profile-freshness alert" and dated titles are a
# different incident and are not closed or reused by a surface recovery.
issue_number() {
  local state="$1" title="$2" raw
  if ! raw="$(gh issue list --repo "$REPO" --state "$state" --limit 100 --json number,title)"; then
    return 2
  fi
  printf '%s' "$raw" | python3 -c 'import json,sys
title=sys.argv[1]
try:
    rows=json.load(sys.stdin)
except json.JSONDecodeError:
    sys.exit(2)
if not isinstance(rows, list):
    sys.exit(2)
for item in rows:
    if isinstance(item, dict) and item.get("title")==title:
        print(item["number"])
        break' "$title"
}

issue_classes() {
  local number="$1" raw
  raw="$(gh issue view "$number" --repo "$REPO" --json body)" || return 2
  printf '%s' "$raw" | python3 -c 'import json,sys
try:
    body=json.load(sys.stdin).get("body") or ""
except json.JSONDecodeError:
    sys.exit(2)
for line in body.splitlines():
    if line.startswith("classes:"):
        print(line.split(":",1)[1].strip())
        break'
}

body_for_classes() {
  local surface="$1" classes="$2" class
  printf 'surface: %s\nclasses: %s\n' "$surface" "$classes"
  for class in $classes; do
    echo
    body_for "$class"
  done
}

maybe_dispatch() {
  local classes="$1" stamp now previous
  [ "$surface" = "local" ] || return 0
  case " $classes " in
    *" workflow_silent "*) ;;
    *) return 0 ;;
  esac
  stamp="${WHERETOKEN_DISPATCH_STAMP:-${HOME}/.config/wheretoken/profile-freshness-dispatch}"
  now="$(python3 -c 'import time; print(int(time.time()))')"
  previous=0
  if [ -f "$stamp" ]; then
    previous="$(python3 -c 'import sys; print(int(open(sys.argv[1]).read() or 0))' "$stamp" 2>/dev/null || printf 0)"
  fi
  if [ $((now - previous)) -lt 10800 ]; then
    echo "dispatch=skipped"
    return 0
  fi
  mkdir -p "$(dirname "$stamp")"
  if gh workflow run profile-freshness.yml --repo "$REPO" --ref main >/dev/null; then
    printf '%s\n' "$now" > "$stamp"
    echo "dispatch=requested"
  else
    echo "dispatch=failed"
  fi
}

notify_surface() {
  local surface="$1" classes="$2" title number body current status
  title="$(surface_title "$surface")"
  classes="$(printf '%s' "$classes" | tr -s '[:space:]' ' ' | sed 's/^ //;s/ $//')"
  if [ -z "$classes" ] || [ "$classes" = "ok" ]; then
    if ! number="$(issue_number open "$title")"; then
      echo "class=check_failed lookup-failed"
      return 1
    fi
    if [ -n "$number" ]; then
      gh issue close "$number" --repo "$REPO" --reason completed --comment "Recovered. surface=${surface}. This is not a new token measurement." >/dev/null
    fi
    echo "class=ok surface=${surface}"
    return 0
  fi
  if ! number="$(issue_number open "$title")"; then
    echo "class=check_failed lookup-failed"
    return 1
  fi
  body="$(body_for_classes "$surface" "$classes")"
  if [ -n "$number" ]; then
    if ! current="$(issue_classes "$number")"; then
      echo "class=check_failed lookup-failed"
      return 1
    fi
    if [ "$current" = "$classes" ]; then
      echo "class=${classes} surface=${surface} issue=${number} already-open"
      maybe_dispatch "$classes"
      return 0
    fi
    gh issue edit "$number" --repo "$REPO" --body "$body" >/dev/null
    echo "class=${classes} surface=${surface} issue=${number} updated"
    maybe_dispatch "$classes"
    return 0
  fi
  if ! number="$(issue_number closed "$title")"; then
    echo "class=check_failed lookup-failed"
    return 1
  fi
  if [ -n "$number" ]; then
    gh issue reopen "$number" --repo "$REPO" >/dev/null
    gh issue edit "$number" --repo "$REPO" --body "$body" >/dev/null
    echo "class=${classes} surface=${surface} issue=${number} reopened"
    maybe_dispatch "$classes"
    return 0
  fi
  gh issue create --repo "$REPO" --title "$title" --body "$body" >/dev/null
  echo "class=${classes} surface=${surface} opened"
  maybe_dispatch "$classes"
}

classify_local() {
  tmp="$(mktemp -d)"
  if ! curl -fsS --retry 3 --retry-delay 5 -H 'Cache-Control: no-cache' -o "$tmp/wall.json" "$WALL_URL"; then
    rm -rf "$tmp"
    echo "check_failed"
    return 0
  fi
  actions_mode="lookup_failed"
  actions_at=""
  if runs="$(gh run list --repo "$REPO" --workflow profile-freshness.yml --limit 10 --json event,createdAt,status,conclusion 2>/dev/null)"; then
    parsed="$(printf '%s' "$runs" | python3 "$HERE/freshness_classify.py" --classify-actions)" || parsed="lookup_failed"
    actions_mode="${parsed%% *}"
    if [ "$parsed" != "$actions_mode" ]; then
      actions_at="${parsed#* }"
    fi
  fi
  set -- python3 "$HERE/freshness_classify.py" --surface local --wall "$tmp/wall.json"
  if [ -f "$STATE_FILE" ]; then
    set -- "$@" --state "$STATE_FILE"
  fi
  case "$actions_mode" in
    ok|running)
      if [ -n "$actions_at" ]; then
        set -- "$@" --workflow-at "$actions_at"
      fi
      ;;
    failed)
      set -- "$@" --workflow-failed
      ;;
    silent)
      set -- "$@" --workflow-missing
      ;;
    *)
      set -- "$@" --actions-lookup-failed
      ;;
  esac
  class="$("$@")"
  rm -rf "$tmp"
  printf '%s\n' "$class"
}

mode="local"
surface=""
classes=""
if [ $# -gt 0 ]; then
  case "$1" in
    --classify-only) mode="classify" ;;
    --surface)
      [ $# -eq 4 ] && [ "$3" = "--classes" ] || usage
      mode="given"
      surface="$2"
      classes="$4"
      ;;
    --drill) mode="drill" ;;
    *) usage ;;
  esac
fi

case "$mode" in
  classify)
    classify_local
    ;;
  given)
    notify_surface "$surface" "$classes"
    ;;
  drill)
    url="$(gh issue create --repo "$REPO" --title "profile-freshness alert drill" --body "Drill. This issue checks that a freshness alert can be opened and closed. The public wall was not marked stale. This is not a new token measurement.")"
    number="${url##*/}"
    gh issue close "$number" --repo "$REPO" --reason completed --comment "Drill complete. Closing immediately. This is not a new token measurement."
    echo "drill=${number} closed"
    ;;
  local)
    surface="local"
    class="$(classify_local)"
    notify_surface local "$class"
    ;;
esac
