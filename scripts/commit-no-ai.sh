#!/bin/sh
# Commit the current index without Cursor Co-authored-by trailers.
# Usage: git add … && scripts/commit-no-ai.sh "subject" ["body"]
set -e
cd "$(git rev-parse --show-toplevel)"
subj=${1:?commit subject required}
body=${2:-}
tree=$(git write-tree)
parent=$(git rev-parse HEAD 2>/dev/null || true)
name=$(git config --get user.name)
email=$(git config --get user.email)
if [ -z "$name" ] || [ -z "$email" ]; then
  echo "git user.name / user.email missing" >&2
  exit 1
fi
export GIT_AUTHOR_NAME="$name"
export GIT_AUTHOR_EMAIL="$email"
export GIT_COMMITTER_NAME="$name"
export GIT_COMMITTER_EMAIL="$email"
if [ -n "$body" ]; then
  msg=$(printf '%s\n\n%s\n' "$subj" "$body")
else
  msg=$(printf '%s\n' "$subj")
fi
if [ -n "$parent" ]; then
  new=$(printf '%s\n' "$msg" | git commit-tree "$tree" -p "$parent")
else
  new=$(printf '%s\n' "$msg" | git commit-tree "$tree")
fi
git reset --soft "$new"
git log -1 --format='%h %s'
