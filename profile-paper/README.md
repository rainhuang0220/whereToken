# Newsprint rendering

The native document owns layout, text, input, scrolling, and accessibility.
A fine-pointer desktop with motion enabled additionally renders a
`PaperFoldSheet` using Three.js in a dedicated Worker. Its UVs cover the full
report, from the header through the technical details.

The controller captures transparent ink in short, cancellable tasks using the
locked html2canvas parser and painter. It sends the capture as an ImageBitmap
along with the document revision, dimensions, DPR, and presentation key.
The Worker retains a cache of scanned stock maps, builds the matching mesh,
uploads the ink, and draws on a detached OffscreenCanvas. The controller
publishes a completed bitmap before paint only if the document, texture, and
mesh still match. Native HTML remains visible during every update.

The flat physical sheet scrolls with the document behind its native text. When
a corner bends, the Worker draws the printed mesh and shadow through a fixed
orthographic camera. Each accepted frame switches to the optical print as
native content becomes transparent. Short pulls recover. Deep folds latch,
can be grabbed again, and can be reopened. A small pressure valley remains in
the mesh after release. In the same tab, it survives changes to the report and
reloads. Leaving Newsprint clears its session state.

Rendering sleeps at rest. Mobile, reduced motion, coarse pointers, and
unavailable Worker WebGL use the same native page and static paper tile.
Cobalt and Magenta do not request the physical renderer.

## Build and verify

```bash
npm --prefix profile-paper ci
npm --prefix profile-paper run build
go test ./internal/profilewebembed ./internal/publicprofile
python3 scripts/profile_pages_test.py
npm --prefix profile-e2e test
```

On Linux, run `WT_LINUX_DESKTOP=1 xvfb-run -a npm --prefix profile-e2e test`
to give Firefox a desktop pointer. The build writes hashed controller and
Worker assets and refreshes the shell references. Both generated public
bundles must use the same embedded assets. `scripts/syncprofileassets` can
regenerate a bundle from its existing sanitized snapshot.
