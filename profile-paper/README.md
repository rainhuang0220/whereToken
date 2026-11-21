# Newspaper: One Always-Physical D4 Sheet

The Newspaper is **not** a CSS-only theme with a temporary folding animation.
On desktop, selecting Newsprint creates a real Three.js D4 paper mesh immediately.
It remains resident and visible during ordinary reading, shading the same
physical paper with scanned fibers, reconstructed normals, roughness and light.
The existing native Profile DOM sits in its original layout directly on that
surface; real glyphs carry a microscopically irregular pigment texture.

When a visitor discovers a sheet edge, the current native report is optically
printed onto the **same resident D4 UV geometry** and folds. During that
movement only, the native DOM becomes visually transparent; it is never
duplicated, detached, moved, or replaced. When the paper recovers, the original
interactive DOM is visible again, while real D4 geometric crease memory remains.
A deeply folded page can be reopened by gripping the actual folded flap.
No frames, separate previews, help labels, special paper controls, or layout
changes exist. This is a physics renderer, not a cloth simulation.

Cobalt and Magenta do not load the 3D mesh. Coarse pointer, reduced motion,
GPU failure or small screen leave the accessible original HTML available.

## Build and verification

```bash
cd profile-paper && npm ci && npm run build && cd ..
go run ./scripts/syncprofileassets --dir public-profile
go run ./scripts/syncprofileassets --dir docs/media/public-profile-demo
go test ./internal/profilewebembed ./internal/publicprofile
cd profile-e2e && npm ci && npx playwright test
```

The build writes `internal/profilewebembed/static/assets/paper-3d.js` and
updates its SHA256 URL in `assets/paper.js`; Go's public profile generator
includes both assets and the ink tile in every subsequent publication. No
external CDN, local-usage logs, credentials or private data are accessed.

D4 folding was designed against the physical paper material rather than a
cloth lattice; it preserves paper UVs and makes finite-width hinge creases.
For a report with controls, the native DOM is always the authoritative view.
