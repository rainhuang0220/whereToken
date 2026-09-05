#!/bin/sh
# Cross-compile the hosted API and the VITE_HOSTED SPA.
set -e
cd "$(git rev-parse --show-toplevel)"
out=${1:-dist/hosted}
mkdir -p "$out"

echo "go linux/amd64"
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -trimpath -ldflags "-s -w" -o "$out/wheretoken-hosted" ./cmd/wheretoken-hosted

echo "web VITE_HOSTED=1"
( cd web && VITE_HOSTED=1 VITE_DEMO= npm run build )
rm -rf "$out/web"
cp -R web/dist "$out/web"
if [ -d "$out/web/sample" ]; then
  echo "hosted dist must not include sample/" >&2
  exit 1
fi
if grep -R "演示数据" "$out/web" >/dev/null 2>&1; then
  echo "hosted dist must not contain 演示数据" >&2
  exit 1
fi
if grep -R "sample/" "$out/web/assets" >/dev/null 2>&1; then
  echo "hosted JS must not fetch sample/" >&2
  exit 1
fi
echo "built $out"
