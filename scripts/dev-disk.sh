#!/bin/sh
# Show disk use for a whereToken checkout and, with --clean, remove only
# caches the next build recreates. This does not delete module downloads,
# Playwright browsers, repository files, or Docker data.
set -eu

echo "disk"
df -h / /tmp "$HOME" 2>/dev/null || df -h /

echo
echo "mysqld processes"
ps aux | awk '/mysqld/ && !/awk/ { printf "%s %s %s\n", $2, $3, $11 }'
echo "A test mysqld whose /tmp datadir was deleted can still fill the disk until that process stops."
echo "Stop only that test process. Leave the Homebrew MySQL server running."

echo
echo "regenerable caches"
for path in \
  "$HOME/Library/Caches/go-build" \
  "$HOME/go/pkg" \
  "$HOME/.npm" \
  "$HOME/Library/Caches/node-gyp" \
  "$HOME/Library/Caches/pnpm" \
  "$HOME/Library/Caches/Homebrew" \
  "$HOME/Library/Caches/ms-playwright"
do
  if [ -e "$path" ]; then
    du -sh "$path"
  fi
done

if [ "${1:-}" != "--clean" ]; then
  echo
  echo "To remove the Go build cache, the npm cache, and Homebrew downloads:"
  echo "  sh scripts/dev-disk.sh --clean"
  echo "Playwright browsers and Go module downloads are left in place."
  exit 0
fi

go clean -cache
if command -v npm >/dev/null 2>&1; then
  npm cache clean --force
fi
if command -v brew >/dev/null 2>&1; then
  brew cleanup -s >/dev/null || true
fi
echo "cleaned go build cache, npm cache, and Homebrew downloads"
