#!/bin/sh
# Cross-compile GitHub Release archives + checksums.txt. Does not publish.
#   bash scripts/package-release.sh 0.1.0 [outdir]
set -e
cd "$(git rev-parse --show-toplevel)"
VERSION=${1:-0.1.0}
VERSION=${VERSION#v}
out=${2:-dist/v${VERSION}}
mkdir -p "$out"

ldflags="-s -w -X main.version=${VERSION}"

STUB_BACKUP=""
cleanup() {
  if [ -n "$STUB_BACKUP" ] && [ -d "$STUB_BACKUP" ]; then
    rm -rf internal/webembed/dist
    mkdir -p internal/webembed/dist
    cp -R "$STUB_BACKUP"/. internal/webembed/dist/
    rm -rf "$STUB_BACKUP"
  fi
}
trap cleanup EXIT

if [ "${WHERETOKEN_EMBED_WEB:-1}" != "0" ] && [ -f web/package.json ]; then
  STUB_BACKUP=$(mktemp -d)
  cp -R internal/webembed/dist/. "$STUB_BACKUP/"
  (cd web && npm ci && npm run build)
  rm -rf internal/webembed/dist
  mkdir -p internal/webembed/dist
  cp -R web/dist/. internal/webembed/dist/
fi

for goos in darwin linux windows; do
  for goarch in amd64 arm64; do
    work=$(mktemp -d)
    bin=wheretoken
    if [ "$goos" = windows ]; then
      bin=wheretoken.exe
    fi
    echo "build ${goos}/${goarch}"
    GOOS="$goos" GOARCH="$goarch" CGO_ENABLED=0 go build -trimpath -ldflags="$ldflags" -o "$work/$bin" ./cmd/wheretoken
    if [ "$goos" = windows ]; then
      (cd "$work" && zip -q "wheretoken_windows_${goarch}.zip" "$bin")
      mv "$work/wheretoken_windows_${goarch}.zip" "$out/"
    else
      tar -C "$work" -czf "$out/wheretoken_${goos}_${goarch}.tar.gz" "$bin"
    fi
    rm -rf "$work"
  done
done

(
  cd "$out"
  if command -v sha256sum >/dev/null 2>&1; then
    sha256sum wheretoken_* > checksums.txt
  else
    shasum -a 256 wheretoken_* > checksums.txt
  fi
)

echo "archives in $out"
ls -lh "$out"
