---
name: wheretoken-release
description: Ship a whereToken 0.4.x patch. Use when tests pass and a generation is stable. Never bump to 0.5.0 unless asked.
---

# Release a 0.4.x

1. `go test ./...` and `go vet ./...`
2. Web: `cd web && npm test` (or the repo's documented web test command)
3. `scripts/verify-cli.sh`
4. Move CHANGELOG Unreleased into `## 0.4.N — YYYY-MM-DD (Alpha)`
5. Bump version in the existing version source only (do not invent a second file)
6. Logical commits, then tag `v0.4.N` after push if that is this repo's convention
7. Do not mark Community Rank as worldwide. Do not claim every AI tool is supported.

Stop if any check fails. Do not delete failing tests to ship.
