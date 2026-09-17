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

1. **Height** — calender the raw displacement toward newsprint (felt kept at low mix; only scan-derived sheet formation, no Fourier cockling).
2. **Normals** — wrapped finite differences, slope RMS locked so the relief stays paper-thin.
3. **Roughness** — valleys and remaining felt are broader / more absorptive; calendered highs keep a tighter sheen.
4. **Lighting** — one soft desk key (upper-left) plus a faint fill, micro-occlusion in fiber pits, and a sub-2% calender sheen. The JPEG is the shaded RGB sheet.
5. **Ink** — not painted into the JPEG. Live Newsprint CSS multiplies existing type, rules, heat cells, rank fills, and the trend over this substrate so ink reduces paper albedo and sits slightly darker in the valleys already carved by the height field.

The previous high-pass-and-tint bake was a paper-looking shader: isotropic fiber noise on `#f7f6f1`. This pass is a printed-paper simulation. Do not reintroduce `feTurbulence`, crease SVGs, or a grayscale soft-light overlay.
