---
name: wheretoken-release
description: Ship a whereToken 0.6.x patch. Use when tests pass and a generation is stable. Do not bump the minor or major version unless asked.
---

# Release a 0.6.x

`docs/releasing.md` is the checklist — gates, tag, goreleaser, and the post-release Formula/tap/npm/site bumps. Follow it. In short:

1. `go test ./...` and `go vet ./...`
2. Web: `cd web && npm test` (or the repo's documented web test command)
3. `scripts/verify-cli.sh`
4. Move CHANGELOG Unreleased into `## 0.6.N — YYYY-MM-DD (Alpha)`
5. Logical commits, then tag `v0.6.N` and push the tag
6. Post-release per `docs/releasing.md`: bump `Formula/wheretoken.rb` (source tarball + freshly computed sha256), the `rainhuang0220/homebrew-wheretoken` tap formula, and `npm/package.json`. The site download button is version-free; leave it alone.
7. Do not mark Community Rank as worldwide. Do not claim every AI tool is supported.

Stop if any check fails. Do not delete failing tests to ship.
