#!/bin/bash
# Exercise wheretoken update from the published v0.7.18 binary to a
# locally built candidate. Does not tag, publish, or touch a Homebrew Cellar.
# WHERETOKEN_HOME is an empty temp directory so macOS refresh rebootstrap
# does not rewrite the owner's LaunchAgent.
set -euo pipefail

root=$(CDPATH= cd -- "$(dirname "$0")/.." && pwd)
work=$(mktemp -d)
trap 'rm -rf "$work"' EXIT

os=$(uname -s)
arch=$(uname -m)
case "$os" in
  Darwin) goos=darwin ;;
  Linux) goos=linux ;;
  MINGW*|MSYS*|CYGWIN*) goos=windows ;;
  *) echo "unsupported OS $os" >&2; exit 2 ;;
esac
case "$arch" in
  arm64|aarch64) goarch=arm64 ;;
  x86_64|amd64) goarch=amd64 ;;
  *) echo "unsupported arch $arch" >&2; exit 2 ;;
esac

if [[ "$goos" == windows ]]; then
  asset="wheretoken_windows_${goarch}.zip"
  binname=wheretoken.exe
else
  asset="wheretoken_${goos}_${goarch}.tar.gz"
  binname=wheretoken
fi
ver=0.7.19-rc
old_url="https://github.com/rainhuang0220/whereToken/releases/download/v0.7.18/${asset}"

py() {
  if command -v python3 >/dev/null 2>&1; then python3 "$@"; else python "$@"; fi
}

echo "candidate upgrade: ${goos}/${goarch} from v0.7.18 to ${ver}"
curl -fsSL "$old_url" -o "$work/old.pkg"
if [[ "$goos" == windows ]]; then
  py -c 'import sys,zipfile; zipfile.ZipFile(sys.argv[1]).extractall(sys.argv[2])' "$work/old.pkg" "$work"
else
  tar -xzf "$work/old.pkg" -C "$work"
fi
old="$work/$binname"
chmod +x "$old"
"$old" --version | grep -q "0.7.18"

mkdir -p "$work/dist"
( cd "$root" && go build -o "$work/dist/$binname" -ldflags "-X main.version=${ver}" ./cmd/wheretoken )
if [[ "$goos" == windows ]]; then
  py -c 'import sys,zipfile; zipfile.ZipFile(sys.argv[1],"w").write(sys.argv[2],"wheretoken.exe")' "$work/dist/${asset}" "$work/dist/$binname"
else
  tar -C "$work/dist" -czf "$work/dist/${asset}" "$binname"
fi
sum=$(openssl dgst -sha256 "$work/dist/${asset}" | awk '{print $NF}')
printf '%s  %s\n' "$sum" "$asset" > "$work/dist/checksums.txt"

port=18765
py -m http.server "$port" --directory "$work/dist" >/tmp/wheretoken-candidate-http.log 2>&1 &
srv=$!
trap 'kill "$srv" 2>/dev/null || true; rm -rf "$work"' EXIT
ready=0
for _ in 1 2 3 4 5 6 7 8 9 10; do
  if curl -fsS "http://127.0.0.1:${port}/checksums.txt" >/dev/null; then
    ready=1
    break
  fi
  sleep 0.2
done
if [[ "$ready" -ne 1 ]]; then
  echo "candidate server did not start" >&2
  exit 1
fi

home="$work/home"
mkdir -p "$home"
export WHERETOKEN_HOME="$home"
export WHERETOKEN_RELEASE_URL="http://127.0.0.1:${port}"

set +e
"$old" update --quiet
code=$?
set -e
if [[ "$code" -ne 0 ]]; then
  echo "update failed: $code" >&2
  exit 1
fi
"$old" --version | grep -q "$ver"
"$old" --json --quiet >"$work/report.json"
grep -q '"schema"' "$work/report.json"

# Idempotent second update.
"$old" update --quiet
"$old" --version | grep -q "$ver"

# A bad checksum must leave the candidate in place and return non-zero.
printf '0000000000000000000000000000000000000000000000000000000000000000  %s\n' "$asset" > "$work/dist/checksums.txt"
set +e
"$old" update --quiet
bad=$?
set -e
if [[ "$bad" -eq 0 ]]; then
  echo "bad checksum was accepted" >&2
  exit 1
fi
"$old" --version | grep -q "$ver"

echo "candidate upgrade PASS ${goos}/${goarch}"
