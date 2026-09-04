# Release checklist

The **git tag** is the single source of truth for a version: goreleaser stamps
`main.version` from it and names every asset after it. The top
`## X.Y.Z —` heading in `CHANGELOG.md` mirrors the tag, and
`internal/cli/version_consistency_test.go` fails CI when the hand-edited
copies (in-repo formula, npm wrapper, public site) drift from it.

Ship `0.6.x` patches. Do not bump the minor or major version unless asked.

## Gates (all green before tagging)

```bash
go test ./...
go vet ./...
cd web && npm ci && npm test
bash scripts/verify-cli.sh
```

## Cut the release

1. Move the CHANGELOG.md unreleased items under a new
   `## X.Y.Z — YYYY-MM-DD (Alpha)` heading.
2. Release-prep commit (`chore(release): vX.Y.Z`), push `main`.
3. `git tag vX.Y.Z && git push origin vX.Y.Z`.
4. The tag triggers `.github/workflows/release.yml` → goreleaser builds the
   six archives + `checksums.txt` + `.deb`/`.rpm` and publishes the GitHub
   Release. macOS signing/notarization runs only when the `MACOS_*` secrets
   are set; otherwise the release is unsigned (README says so).

## Post-release (same day, in this order)

1. **In-repo formula** — `Formula/wheretoken.rb` is a source build. Compute
   the new tarball hash, then update `url` and `sha256`:

   ```bash
   curl -sL https://github.com/rainhuang0220/whereToken/archive/refs/tags/vX.Y.Z.tar.gz | shasum -a 256
   ```

   Never invent a checksum.
2. **Tap formula** — `rainhuang0220/homebrew-wheretoken` is a separate repo
   with a *binary* formula. Goreleaser has no `brews:` section, so this is a
   manual step: pull `checksums.txt` from the new release and update the
   `url`/`sha256` pairs for `wheretoken_{darwin,linux}_{amd64,arm64}.tar.gz`:

   ```bash
   curl -sL https://github.com/rainhuang0220/whereToken/releases/download/vX.Y.Z/checksums.txt
   ```

   Until the tap is bumped, brew users stay on the old version and
   `wheretoken update` (which defers to `brew upgrade` under
   `/Cellar/wheretoken/`) is a no-op for them.
3. **npm wrapper** — bump `version` in `npm/package.json` to `X.Y.Z`; the
   postinstall fetches `releases/download/v<pkg.version>`, so the version
   must track the release. The package is deliberately **not on the npm
   registry** — do not `npm publish` unless someone with registry access
   decides to make it a real channel.
4. **Site** — the `site/index.html` download button is version-free
   ("Download latest") with `href` on `releases/latest`; confirm it still is,
   no edit needed.
5. **Pages smoke** — pushes to `main` touching `site/`/`web/` deploy via
   `ci/github-workflows/pages.yml`. After the deploy, load the site, `/demo/`,
   and the custom domain if one is configured (Pages custom domains are set
   repo-side, unmanaged by CI).
6. **Dogfood the release binary** (per platform you can reach):

   ```bash
   curl -sLO https://github.com/rainhuang0220/whereToken/releases/download/vX.Y.Z/wheretoken_darwin_arm64.tar.gz
   curl -sLO https://github.com/rainhuang0220/whereToken/releases/download/vX.Y.Z/checksums.txt
   shasum -a 256 -c checksums.txt --ignore-missing
   tar xzf wheretoken_darwin_arm64.tar.gz
   ./wheretoken --version   # must print: wheretoken X.Y.Z
   ./wheretoken             # runs against your real ledger
   ```

   `--version` alone is not enough — the terminal KPI is the product surface
   that once shipped stale while the web dashboard passed. Assert it on the
   unpacked binary (web tests never cover the CLI renderer):

   ```bash
   out="$(./wheretoken --quiet)"
   echo "$out" | grep -q '用户画像'                  # bottom-right KPI cell
   ! echo "$out" | grep -q '排名'                   # rank is gone from the report
   ! echo "$out" | grep -q '社区排名暂不可用'        # and so is its note
   echo "$out" | grep -Eq '\$[0-9]{1,3}(,[0-9]{3})*\.[0-9]{2}'   # 2-dec grouped estimate (when priced)
   ```

   `bash scripts/verify-cli.sh` makes the same assertions against a fresh
   `go build` of `./cmd/wheretoken`, so CI gates catch this before the tag;
   the dogfood catches a packaging drift after it.

Stop if any check fails. Do not delete failing tests to ship.
