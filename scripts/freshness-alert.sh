#!/bin/bash
# Classify the public wall and open or close one GitHub issue.
# A fresh result closes the open alert. A bad result opens one issue
# and does not comment again while that issue stays open.
set -euo pipefail

REPO="${GITHUB_REPOSITORY:-rainhuang0220/whereToken}"
TITLE_PREFIX="profile-freshness alert"
# GitHub-hosted workflow time is UTC; this date identifies a unique incident
# day. The stable prefix, not the full date, deduplicates an ongoing alert.
TITLE="${TITLE_PREFIX} ($(date -u +%Y-%m-%d))"
WALL_URL="${WHERETOKEN_WALL_URL:-https://wheretoken.plainlist.space/api/v1/public-profile/rainhuang0220}"
STATE_FILE="${WHERETOKEN_REFRESH_STATE:-${HOME}/.config/wheretoken/profile-refresh-state.json}"
HERE="$(cd "$(dirname "$0")" && pwd)"

usage() {
  echo "usage: freshness-alert.sh [--classify-only | --class CLASS | --drill]" >&2
  exit 2
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

The public wall is older than 48 hours and this Mac has no publisher check in the last 6 hours.
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
    *)
      echo "class: $1"
      echo
      echo "This is not a new token measurement."
      ;;
  esac
}

open_number() {
  # Keep the original one-open-issue semantics even across UTC date rollover.
  # Legacy undated issues remain recognized until recovered.
  gh issue list --repo "$REPO" --state open --limit 100 --json number,title \
    | python3 -c 'import json,sys
prefix=sys.argv[1]
for item in json.load(sys.stdin):
    title=item.get("title","")
    if title==prefix or title.startswith(prefix+" ("):
        print(item["number"])
        break' "$TITLE_PREFIX"
}

closed_number_today() {
  # If a check fails again after recovering on the same day, reopen that
  # issue instead of creating repeated same-date alert tickets.
  gh issue list --repo "$REPO" --state closed --limit 100 --json number,title \
    | python3 -c 'import json,sys
title=sys.argv[1]
for item in json.load(sys.stdin):
    if item.get("title")==title:
        print(item["number"])
        break' "$TITLE"
}

notify() {
  local class="$1"
  if [ "$class" = "ok" ]; then
    local number
    number="$(open_number || true)"
    if [ -n "${number}" ]; then
      gh issue close "$number" --repo "$REPO" --reason completed --comment "Recovered. The freshness check is ok. This is not a new token measurement."
    fi
    echo "class=ok"
    return 0
  fi
  local number
  number="$(open_number)"
  if [ -n "${number}" ]; then
    echo "class=${class} issue=${number} already-open"
    return 0
  fi
  number="$(closed_number_today)"
  if [ -n "${number}" ]; then
    gh issue reopen "$number" --repo "$REPO" >/dev/null
    echo "class=${class} issue=${number} reopened"
    return 0
  fi
  local body
  body="$(body_for "$class")"
  gh issue create --repo "$REPO" --title "$TITLE" --body "$body" >/dev/null
  echo "class=${class} opened"
}

classify_local() {
  tmp="$(mktemp -d)"
  if ! curl -fsS --retry 3 --retry-delay 5 -H 'Cache-Control: no-cache' -o "$tmp/wall.json" "$WALL_URL"; then
    rm -rf "$tmp"
    echo "check_failed"
    return 0
  fi
  workflow_at=""
  if runs="$(gh run list --repo "$REPO" --workflow profile-freshness.yml --limit 10 --json event,createdAt,status 2>/dev/null)"; then
    workflow_at="$(printf '%s' "$runs" | python3 -c 'import json,sys
rows=json.load(sys.stdin)
for row in rows:
    if row.get("event") in ("schedule","workflow_dispatch") and row.get("createdAt"):
        print(row["createdAt"])
        break')"
  fi
  set -- python3 "$HERE/freshness_classify.py" --surface local --wall "$tmp/wall.json"
  if [ -f "$STATE_FILE" ]; then
    set -- "$@" --state "$STATE_FILE"
  fi
  if [ -n "$workflow_at" ]; then
    set -- "$@" --workflow-at "$workflow_at"
  fi
  class="$("$@")"
  rm -rf "$tmp"
  printf '%s\n' "$class"
}

mode="local"
class_arg=""
if [ $# -gt 0 ]; then
  case "$1" in
    --classify-only) mode="classify" ;;
    --class)
      [ $# -eq 2 ] || usage
      mode="given"
      class_arg="$2"
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
    notify "$class_arg"
    ;;
  drill)
    url="$(gh issue create --repo "$REPO" --title "profile-freshness alert drill" --body "Drill. This issue checks that a freshness alert can be opened and closed. The public wall was not marked stale. This is not a new token measurement.")"
    number="${url##*/}"
    gh issue close "$number" --repo "$REPO" --reason completed --comment "Drill complete. Closing immediately. This is not a new token measurement."
    echo "drill=${number} closed"
    ;;
  local)
    class="$(classify_local)"
    notify "$class"
    ;;
esac
