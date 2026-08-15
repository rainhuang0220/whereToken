#!/usr/bin/env bash
# Install wheretoken from GitHub Releases. No Go required.
#   curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
# Optional: PREFIX=$HOME/.local WHERETOKEN_VERSION=0.1.0 bash scripts/install.sh
set -euo pipefail

REPO="${WHERETOKEN_REPO:-rainhuang0220/whereToken}"
PREFIX="${PREFIX:-$HOME/.local}"
BIN_DIR="${BIN_DIR:-$PREFIX/bin}"

need() {
  if ! command -v "$1" >/dev/null 2>&1; then
    echo "wheretoken: need $1 on PATH" >&2
    exit 1
  fi
}

need curl
need tar
need uname
need mkdir

os=$(uname -s | tr '[:upper:]' '[:lower:]')
arch=$(uname -m)
case "$os" in
  darwin) os=darwin ;;
  linux) os=linux ;;
  mingw*|msys*|cygwin*)
    echo "wheretoken: on Windows use npm install -g wheretoken or go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest" >&2
    exit 1
    ;;
  *)
    echo "wheretoken: unsupported OS $os" >&2
    exit 1
    ;;
esac
case "$arch" in
  x86_64|amd64) arch=amd64 ;;
  arm64|aarch64) arch=arm64 ;;
  *)
    echo "wheretoken: unsupported arch $arch" >&2
    exit 1
    ;;
esac

asset="wheretoken_${os}_${arch}.tar.gz"

version="${WHERETOKEN_VERSION:-}"
if [ -z "$version" ]; then
  version=$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | tr -d '\r' | sed -n 's/.*"tag_name": *"v\{0,1\}\([^"]*\)".*/\1/p' | head -n 1 || true)
fi
version="${version#v}"

if [ -z "$version" ]; then
  echo "wheretoken: no GitHub Release yet. Install with Go:" >&2
  echo "  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest" >&2
  exit 1
fi

url="https://github.com/${REPO}/releases/download/v${version}/${asset}"
tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "wheretoken: downloading ${url}" >&2
if ! curl -fsSL -o "$tmp/$asset" "$url"; then
  echo "wheretoken: download failed. Try:" >&2
  echo "  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest" >&2
  exit 1
fi
sums_url="https://github.com/${REPO}/releases/download/v${version}/checksums.txt"
if ! curl -fsSL -o "$tmp/checksums.txt" "$sums_url"; then
  echo "wheretoken: no checksums.txt for v${version}; refusing to install" >&2
  echo "  go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest" >&2
  exit 1
fi
if command -v sha256sum >/dev/null 2>&1; then
  (cd "$tmp" && grep -F "$asset" checksums.txt | sha256sum -c -)
elif command -v shasum >/dev/null 2>&1; then
  (cd "$tmp" && grep -F "$asset" checksums.txt | shasum -a 256 -c -)
else
  echo "wheretoken: need sha256sum or shasum to verify the download" >&2
  exit 1
fi
tar -xzf "$tmp/$asset" -C "$tmp"
bin=""
if [ -f "$tmp/wheretoken" ]; then
  bin="$tmp/wheretoken"
else
  bin=$(find "$tmp" -type f -name wheretoken | head -n 1)
fi
if [ -z "$bin" ]; then
  echo "wheretoken: archive had no binary" >&2
  exit 1
fi
mkdir -p "$BIN_DIR"
install -m 0755 "$bin" "$BIN_DIR/wheretoken"
echo "wheretoken: installed $BIN_DIR/wheretoken" >&2
if ! command -v wheretoken >/dev/null 2>&1; then
  echo "wheretoken: add $BIN_DIR to PATH" >&2
fi
"$BIN_DIR/wheretoken" --version || true
