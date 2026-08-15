#!/bin/sh
# Cross-compile the CLI. Does not publish.
set -e
cd "$(git rev-parse --show-toplevel)"
out="${TMPDIR:-/tmp}/wheretoken-dist"
mkdir -p "$out"
ok=0
for goos in darwin linux windows; do
  for goarch in amd64 arm64; do
    ext=""
    if [ "$goos" = windows ]; then ext=.exe; fi
    name="wheretoken_${goos}_${goarch}${ext}"
    echo "build $name"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags='-s -w' -o "$out/$name" ./cmd/wheretoken
    ok=$((ok+1))
  done
done
echo "built $ok binaries in $out"
ls -lh "$out"
