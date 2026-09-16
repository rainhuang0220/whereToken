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

`go run ./scripts/gennewsprint` retints an albedo-led fiber mix onto a near-white sheet and writes `newsprint-surface.jpg`. Displacement is a quiet extra tooth, not a page-scale lighting field.
