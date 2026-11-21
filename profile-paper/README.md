# Newspaper as a Single Native Profile

Newsprint is an **ink-and-paper palette** applied to whereToken's *existing*
accessible Profile DOM. It does not create a second report, duplicated snapshot
or framed Three.js preview. Switching Cobalt, Magenta and Newsprint preserves
all layout measurements and all navigation, filters, charts, cells, links and
screen-reader semantics.

## The hidden D4 fold

On a desktop fine-pointer device, grabbing an otherwise unmarked paper corner
or side edge momentarily captures the currently visible native report with a
self-hosted html2canvas renderer. D4 maps that real report (including its
current content and activity heatmap) into a Three.js continuous-sheet fold.
The original DOM occupies the same coordinates and is simply made transparent
while the fold is active. On return, the native DOM immediately takes over.
Escape or a click outside the folded paper restores the page. The physical
model is not a full FEM or self-collision simulator.

This behavior is an Easter egg, not a second UI: no display stand, button,
label, border, dedicated scroll pane, or extra document heading is added.

Text uses a real ink-density raster tile with irregular pinholes and variance
inside its strokes. The source paper scan remains CC0 ambientCG Paper001.
The 3D engine runs only on demand and is not loaded for Cobalt/Magenta.

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
