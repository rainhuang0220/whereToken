# Newspaper: one whole, always-printed D4 document

Desktop Newsprint is a single continuous Three.js `PaperFoldSheet`, not a CSS
theme, a central card, or a temporary folding animation. Its UV space covers the
viewport width and the document's complete scroll height, including the header,
controls, activity, breakdown, and technical details. A fixed viewport canvas
renders that document-sized mesh; scrolling moves only the orthographic camera.

The actual current HTML report is captured after data renders and composited with
the scanned paper's optical absorption, fiber normals, and roughness. This
printed front material remains visible at rest, throughout dragging, after
recovery, and while latched. The back is blank paper. Ink stays fixed to the D4
UVs, and residual creases are geometric valleys with asymmetric shoulders.

The native semantic DOM remains in exactly the same layout above the canvas.
After capture its report visuals become transparent, never detached or
`display: none`, so pointer targets, keyboard navigation, selection, and
accessibility remain native. Hovered or focused controls and tooltips can appear
natively above the print. Before capture and after any invalidation, ordinary
HTML is visible instead of stale physical ink.

All outer document corners and edges use the approved trajectory- and
speed-sensitive D4 hinge. Short pulls recover; deep folds latch and can be
re-grabbed at the projected flap and reopened. Reopening does not count the
initial latched angle as a new fold. Geometric crease memories survive release.
Wheel events are not trapped. Reduced motion, coarse pointers, small screens,
and WebGL failure or context loss retain the readable HTML fallback.

Rendering is invalidation-driven: there is no continuous animation frame loop at
rest and no DOM capture on scroll or pointer movement. Cobalt and Magenta do not
load the 3D bundle. No runtime CDN or additional dependency is used.

## Build and verification

```bash
cd profile-paper && npm ci && npm run build && cd ..
go run ./scripts/syncprofileassets --dir public-profile
go run ./scripts/syncprofileassets --dir docs/media/public-profile-demo
go test ./internal/profilewebembed ./internal/publicprofile
python3 scripts/profile_pages_test.py
cd profile-e2e && npx playwright test
```

The build writes `internal/profilewebembed/static/assets/paper-3d.js` and updates
its SHA256 URL in `assets/paper.js`. The sync command refreshes content-addressed
assets in both generated public bundles. Rendering diagnostics, full-document
layout comparisons, physical mouse regressions, fallback checks, and review
screenshots verify the result without accessing source token data.
