#!/usr/bin/env bash
# Install wheretoken from GitHub Releases. No Go required.
#   curl -fsSL https://raw.githubusercontent.com/rainhuang0220/whereToken/main/scripts/install.sh | bash
# Optional: PREFIX=$HOME/.local WHERETOKEN_VERSION=0.1.0 bash scripts/install.sh
set -euo pipefail

REPO="${WHERETOKEN_REPO:-rainhuang0220/whereToken}"

path_has() {
  case ":$PATH:" in
    *:"$1":*) return 0 ;;
    *) return 1 ;;
  esac
}

choose_bin_dir() {
  if [ -n "${BIN_DIR:-}" ]; then
    printf '%s\n' "$BIN_DIR"
    return
  fi
  if [ -n "${PREFIX:-}" ]; then
    printf '%s\n' "$PREFIX/bin"
    return
  fi
  local_default="$HOME/.local/bin"
  if path_has "$local_default"; then
    printf '%s\n' "$local_default"
    return
  fi
  if [ -d /usr/local/bin ] && [ -w /usr/local/bin ]; then
    printf '%s\n' /usr/local/bin
    return
  fi
  printf '%s\n' "$local_default"
}

BIN_DIR="$(choose_bin_dir)"

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
    echo "wheretoken: on Windows use scripts/install.ps1" >&2
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
version="${version#v}"

if [ -n "${WHERETOKEN_RELEASE_URL:-}" ]; then
  base="${WHERETOKEN_RELEASE_URL%/}"
elif [ -n "$version" ]; then
  base="https://github.com/${REPO}/releases/download/v${version}"
else
  base="https://github.com/${REPO}/releases/latest/download"
fi

url="${base}/${asset}"
sums_url="${base}/checksums.txt"

remember_path() {
  local dir="$1"
  [ -n "${HOME:-}" ] || return 0
  local line="export PATH=\"${dir}:\$PATH\""
  local rc="${HOME}/.zshrc"
  if [ -n "${ZDOTDIR:-}" ]; then
    rc="${ZDOTDIR}/.zshrc"
  fi
  if { [ -f "$rc" ] && grep -F "$dir" "$rc" >/dev/null 2>&1; } ||
    { [ -f "${HOME}/.zprofile" ] && grep -F "$dir" "${HOME}/.zprofile" >/dev/null 2>&1; }; then
    return 0
  fi
  printf '\n# whereToken\n%s\n' "$line" >>"$rc"
  echo "wheretoken: added ${dir} to PATH in ${rc} (new terminals)" >&2
}

finish() {
  if ! path_has "$BIN_DIR"; then
    remember_path "$BIN_DIR"
  fi
  ver=$("${BIN_DIR}/wheretoken" --version 2>/dev/null || true)
  if [ -n "$ver" ]; then
    echo "wheretoken: installed ${ver}" >&2
  else
    echo "wheretoken: installed" >&2
  fi
  echo "${BIN_DIR}/wheretoken" >&2
}

go_fallback() {
  if ! command -v go >/dev/null 2>&1; then
    echo "wheretoken: download failed (${url})" >&2
    exit 1
  fi
  echo "wheretoken: no GitHub Release; installing with go install" >&2
  mkdir -p "$BIN_DIR"
  local ref=latest
  if [ -n "$version" ]; then
    ref="v${version}"
  fi
  GOBIN="$BIN_DIR" go install "github.com/rainhuang0220/whereToken/cmd/wheretoken@${ref}"
  finish
  exit 0
}

tmp=$(mktemp -d)
trap 'rm -rf "$tmp"' EXIT
echo "wheretoken: downloading ${url}" >&2
if ! curl -fsSL -A wheretoken-install -o "$tmp/$asset" "$url"; then
  if [ -n "${WHERETOKEN_RELEASE_URL:-}" ]; then
    echo "wheretoken: download failed" >&2
    exit 1
  fi
  go_fallback
fi
if ! curl -fsSL -A wheretoken-install -o "$tmp/checksums.txt" "$sums_url"; then
  echo "wheretoken: no checksums.txt; refusing to install" >&2
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
if command -v install >/dev/null 2>&1; then
  install -m 0755 "$bin" "$BIN_DIR/wheretoken"
else
  cp "$bin" "$BIN_DIR/wheretoken"
  chmod 0755 "$BIN_DIR/wheretoken"
fi
finish
