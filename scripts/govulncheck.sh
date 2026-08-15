#!/usr/bin/env bash
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"
if ! command -v govulncheck >/dev/null 2>&1; then
  go install golang.org/x/vuln/cmd/govulncheck@latest
fi
export PATH="$(go env GOPATH)/bin:$PATH"
govulncheck ./...
