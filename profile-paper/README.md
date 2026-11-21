# Physical Newsprint Front Page

whereToken's Newsprint palette uses a self-hosted Three.js D4 front page. The original accessible HTML dashboard remains below it and continues to provide detailed interactive ranges, metrics, source filtering, export, and publication controls. Cobalt and Magenta do not load the 3D engine.

## Rebuild

From the repository root:

1. Run `cd profile-paper && npm ci && npm run build && cd ..`
2. Run `go run ./scripts/syncprofileassets --dir public-profile`
3. Run `go run ./scripts/syncprofileassets --dir docs/media/public-profile-demo`
4. Verify with `go test ./internal/profilewebembed ./internal/publicprofile` and the Playwright tests in `profile-e2e`

The engine bundle is committed at `internal/profilewebembed/static/assets/paper-3d.js`. The build updates its SHA-256 content-addressed URL in `assets/paper.js`. The Go `BundleWith` generator copies the asset into every public-profile publication and includes it in the asset-revision hash. An older installed whereToken publisher still embeds its old assets: upgrade or rebuild the publisher before allowing it to overwrite the newly deployed profile bundle.

## Data and safety

The print renderer sees only the already-sanitized public snapshot and selected metric/range/agent. It never reads private logs, file paths, account auth tokens, or raw prompts. Production print is labeled `PUBLIC SNAPSHOT`; synthetic demos remain labeled `SYNTHETIC DEMO`. The exact original snapshot date is preserved. A rendered newspaper front page is read-only; the accessible, interactive HTML dashboard remains authoritative.

The physical interaction is a single continuous-sheet geometric hinge model with finite-radius bending, a blank paper backside, and residual geometric crease memory. It is not a calibrated elastic-plastic finite-element simulation; multiple fold collisions are outside its scope.

Small screens, coarse pointers, reduced-motion requests, failed GPU initialization, or blocked canvas access always retain the complete HTML view. The bundle is loaded lazily only for visitors who select Newsprint.

## Upstream license notes

- Three.js is distributed under the MIT license.
- The existing Paper001 scanned material from ambientCG is CC0; see `scripts/gennewsprint/vendor/SOURCE.md`.

No runtime CDN, tracker or third-party network request is required for the physical renderer.
