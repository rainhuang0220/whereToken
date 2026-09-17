# Newsprint material source

Production Newsprint is a local adaptation of a photoscanned paper material.

## Upstream

- Asset: [Paper001](https://ambientcg.com/a/Paper001) (“Paper 001”, tagged white paper)
- Library: [ambientCG](https://ambientcg.com/)
- License: [CC0 1.0 Universal](https://creativecommons.org/publicdomain/zero/1.0/) as stated on the [ambientCG license page](https://ambientcg.com/license)
- Capture: height-field photogrammetry (Color + Displacement maps)
- Pack downloaded: `Paper001_2K-JPG.zip` from `https://ambientcg.com/get?file=Paper001_2K-JPG.zip`
- Downloaded: 2026-09-17

SHA-256:

```
21b3b955d9cd8bd6c83ebac6a71f7dcbaea819d28b6a0f82a1a3805f67793dcb  Paper001_2K-JPG.zip
1ec1da7815c3e53e02f7f6f4c68b091dddd5d089ba9aa40ccebb7685e3032804  Paper001_2K-JPG_Color.jpg
579cf1f3984db5e7e210e2684179fca7d362d3e272388e283fc0ffd7166ab7b2  Paper001_2K-JPG_Displacement.jpg
```

Poly Haven has no paper category in its texture catalog. Paper Shaders remain a parameter reference only (fiber present, folds/crumples off). This material does not use `feTurbulence` as the source of paper.

## Local maps

`Paper001_Color.jpg` and `Paper001_Displacement.jpg` in this directory are still CC0. They are a cropped, seam-blended, 1600×997 working extract of the 2K Color and Displacement maps. The damaged right strip of the original scan is removed.

## Bake

`go run ./scripts/gennewsprint` treats the photoscanned displacement as a **height field**, not as a grayscale overlay.

1. **Height** — calender Paper001's watercolor tooth toward machine-finished newsprint. Add only the scan's real mid-frequency pulp (no Fourier, and no contrast-normalization of low-frequency — that reconstructs the rejected cloudy field).
2. **Normals** — wrapped finite differences with a fixed paper-thin slope gain. Do not RMS-lock: that re-amplifies leftover pebbles into a tiled shader stamp.
3. **Albedo** — remaining tooth is a paper-color change from the same displacement, not a bump film. Scanned speckle stays quiet.
4. **Roughness / lighting** — almost uniformly matte; one soft desk key, faint fill, tiny occlusion, sub-1% sheen. The JPEG is the shaded RGB sheet.
5. **Ink** — not painted into the JPEG. Live Newsprint CSS multiplies existing type, rules, heat cells, rank fills, and the trend over this substrate so ink reduces paper albedo.

The previous height-lit bake still read as a watercolor-paper website: uniformly loud tooth, lighting as a tiled JPEG stamp. This pass is a newsprint-specific paper model from the same scan. Do not reintroduce `feTurbulence`, crease SVGs, or a grayscale soft-light overlay.
