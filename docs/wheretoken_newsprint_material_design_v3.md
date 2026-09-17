# whereToken Newsprint Material Design v3

> Status: superseded as a production bake. The v3 *procedural Fourier* height field was rejected (obvious, cloudy, not aesthetically acceptable). Production Newsprint still follows the PBR-lite *stack* specified here — height, normals, roughness, desk-key lighting, ink as a separate absorbing layer — but the height source is photoscanned ambientCG Paper001 displacement, not synthesized cockling. See `scripts/gennewsprint/vendor/SOURCE.md`.
>
> Iteration 2 (printed-paper simulation): `scripts/gennewsprint` reconstructs normals from the scanned height, varies roughness by valley/calender, lights a shaded RGB sheet, and the live page multiplies existing inked pixels over that substrate. It does not retile a fiber JPEG as a grayscale overlay.
>
> Status: implementation specification
> Scope: only the `Newsprint` theme of the live public profile
> Baseline inspected: `7d9a1629efef67fdad5954a12e5563f382175a35` (`fix(profile): render newsprint as procedural paper`)
> Intended coding model: GPT-5.6 Sol · Medium

## 0. Executive decision

The previous two Newsprint attempts used the wrong visual model.

**Newsprint is not “white page + one or two folds + noise”.** A real newspaper sheet reads as paper because of a *continuous, multiscale material field*: substrate color, micro-fiber/roughness, meso-scale waviness/cockling, low-gloss lighting, and the way ink sits in an absorbent uncoated surface. A crease is a discrete event; paper texture is a distributed surface property.

Therefore the next implementation must:

1. **Delete the explicit crease-path model** as the defining effect.
2. Build a **PBR-lite, height-field-based paper material**.
3. Use **broad stochastic cockling/waviness**, not identifiable lines.
4. Keep **micro fiber/roughness** as a separate frequency band.
5. Add a very restrained **ink-mottle/absorption cue** to large filled data marks.
6. **Bake the material to static assets** for production rather than ship a live WebGL/Three.js physics renderer.
7. Preserve the accepted layout, theme semantics, token intensity formula, interactions, and Cobalt/Magenta themes.

The desired impression at 100% browser zoom is:

> “This whole page is printed on a slightly wavy, matte, fibrous sheet of fresh newsprint.”

Not:

> “Someone drew wrinkles on a website.”

---

# 1. What is wrong with the current implementation

The current canonical assets reveal why the result looks artificial.

### 1.1 `newsprint-paper.svg`

The existing paper texture is primarily:

- high-frequency `feTurbulence/fractalNoise`;
- one low-frequency turbulence layer;
- one large radial dark field;
- one large radial light field;
- those layers multiplied into the background at low opacity.

This creates **tonal noise**, but not a physical surface. There is no common height field from which surface orientation and lighting are derived. A random light/dark pixel is therefore just a random light/dark pixel; it is not perceived as a local change in paper normal.

### 1.2 `newsprint-folds.svg`

The current folds asset is structurally:

- one long near-vertical Bézier path;
- one long diagonal Bézier path;
- each path duplicated as blur + dark core + highlight.

Changing those paths from straight to curved cannot solve the problem. The eye still recognizes them as **two illustrated lines**.

This is the central design error.

### 1.3 `body::before`

The current page overlays the fold SVG as a separate layer. That reinforces the “effect pasted on top” feeling. The substrate and its deformation are not one material.

### 1.4 Corrective action

For this pass:

- remove `newsprint-folds.svg` from the production material system;
- remove `--paper-crease`, `--paper-crease-size`, and the Newsprint crease overlay;
- do **not** replace them with three, five, or ten new paths;
- do **not** add “micro wrinkles” as more SVG strokes.

A successful result must still read as paper when there are **zero explicit crease paths**.

---

# 2. What physical paper actually gives us visually

## 2.1 Paper is multiscale

Surface characterization work treats paper topography across multiple spatial scales rather than as a single noise texture. Broad form/waviness, cockling, and smaller roughness are distinguishable parts of the surface.

For our screen simulation, use four perceptual bands:

| Band | Physical idea | Screen cue | Approx. CSS scale |
|---|---|---|---|
| Substrate | paper shade / formation | base off-white, very low-frequency density variation | whole viewport / 300–900 px |
| Meso | cockling / waviness | broad irregular hills and valleys | ~25–180 px |
| Micro | fibers / roughness | fine anisotropic tactile grain | ~1–7 px |
| Ink | absorption / mottle | tiny density irregularity inside large inked regions | ~4–30 px |

The exact physical-to-screen scale is an artistic mapping, not a literal conversion, but the **separation of frequency bands is mandatory**.

## 2.2 Cockling is the key missing cue

Cockling is planar distortion that appears as wrinkles, ripples or puckers and can exist **without creases**. Published descriptions of cockled sheets discuss randomly spaced out-of-plane areas on roughly 5–50 mm scales.

That is the right mental model for whereToken.

The page should contain many broad, overlapping, low-amplitude height changes. No one ridge should announce itself as “the fold”.

## 2.3 Real newsprint is matte and not screen-white

Commercial newsprint is uncoated and matte. For example, UPM News C lists:

- D65 brightness around 58;
- L* around 83.4;
- positive b* around 2.9–4.2;
- substantial measured roughness.

A literal conversion of L*=83.4 to our UI would make the page far too dark and dirty. We should **screen-adapt** it, not copy the measurement.

Recommended base target:

- initial CSS paper base: **`#F2F0E9`**
- allowed tuning range: approximately `#F0EEE7` → `#F5F3ED`
- avoid yellow/sepia; the warm component should be barely perceptible.

This is deliberately more material than the current `#FEFEFC`.

## 2.4 Newsprint is low gloss

Do not add shiny specular highlights. The material should be almost entirely diffuse. Surface shape appears through extremely shallow diffuse shading.

## 2.5 Ink contributes to the newspaper impression

Coldset newspaper ink sets largely by absorption into uncoated paper. Print-mottle literature describes laterally varying stochastic graininess/cloudiness caused partly by surface roughness and uneven ink setting/absorption.

We should not blur all text or make fake damaged type. Text clarity is more important.

Use the ink cue only where it helps:

- heatmap cells;
- rank usage bars;
- possibly the activity line at extremely low amplitude.

---

# 3. Mature implementation references and what to learn from them

## 3.1 Paper Design — Paper Texture shader

This is the most useful implementation reference found.

Its mature paper shader does **not** treat “paper texture” as one noise and a couple of folds. It exposes separate controls for:

- roughness;
- fiber + fiber size;
- crumples + crumple size;
- folds + fold count;
- large-scale fade;
- drops/speckles;
- seed.

The shader source also computes roughness, fiber, crumples, and folds separately before converting them into a surface normal/light response.

### What we borrow conceptually

- material = several independent frequency families;
- roughness/fiber and crumpling are not the same phenomenon;
- use a surface-normal/light model rather than painting shadows directly;
- use deterministic seeded generation.

### What we explicitly do NOT borrow

- do not ship its runtime React/WebGL component just to make a background;
- do not make folds central;
- for our target, prototype with **`folds = 0`**;
- do not copy its shader source verbatim unless licensing/NOTICE requirements are intentionally handled.

It is Apache-2.0 licensed, so direct reuse is possible with compliance, but conceptual reimplementation/baking is cleaner for this repository.

## 3.2 SVG lighting primitives

SVG supports `feDiffuseLighting`, which treats an input as a bump map and derives lighting from surface geometry. `feTurbulence` and `feDisplacementMap` are also broadly implemented.

This is useful for a prototype and for the high-frequency fiber tile.

However, browser SVG filters can vary subtly by renderer, and a full-page live filter may be unnecessarily expensive. Therefore they are better used as:

- an authoring/prototyping path; or
- a small static/tile asset;

not a permanent dynamic filter on the entire body.

## 3.3 PBR material workflows

Professional PBR workflows separate:

- albedo/base color;
- displacement/height;
- normal;
- roughness;
- AO.

The lesson for this tiny web page is not “ship five 8K maps”. The lesson is **separation of concerns**:

- substrate color is not lighting;
- surface shape is not color noise;
- roughness is not a hand-painted dark streak.

Our PBR-lite solution can compress this into one baked shading map + one fine fiber layer.

---

# 4. Chosen architecture: static PBR-lite material

## 4.1 Why not full 3D / Three.js

A true mesh simulation is unnecessary:

- no user interaction requires physical deformation;
- the texture is static;
- runtime WebGL introduces context, memory, accessibility, failure, and screenshot differences;
- whereToken already has a simple static profile architecture.

## 4.2 Why not AI-generated image

A generated image is the wrong default because:

- it bakes arbitrary lighting into pixels;
- it is hard to tile;
- its physical scale is uncontrolled;
- it may contain fake macro features;
- changing base tone later requires regeneration;
- a 1440/4K page may expose texture resolution or repetition;
- it is difficult to guarantee deterministic source-of-truth.

AI image generation remains last resort only.

## 4.3 Why not the current all-SVG path model

Explicit vector paths are exactly what failed perceptually.

## 4.4 Recommended production architecture

Use **two static material assets plus CSS base**:

### Layer A — CSS substrate

`--newsprint-base: #F2F0E9` initially.

This is the albedo/substrate tone.

### Layer B — baked low/mid-frequency surface shading

Asset concept:

`newsprint-surface.png`
(or WebP only if the existing generation/deployment toolchain already has a reliable encoder; do not add a production dependency merely for WebP)

Properties:

- deterministic fixed seed;
- seamless/tile-safe;
- grayscale;
- contains **only surface-light response**, not text, fibers, borders, or explicit folds;
- generated from a continuous height field;
- target asset size: 1536×1536 or 2048×2048;
- low/mid-frequency enough that PNG compresses reasonably;
- `background-blend-mode: multiply` / carefully controlled opacity;
- no line-shaped feature extending conspicuously through the viewport.

### Layer C — micro fiber/roughness

`newsprint-fiber.svg`

Properties:

- seamless;
- fine-scale;
- deterministic;
- slight directional anisotropy to suggest machine-made fibers;
- extremely low contrast;
- fixed CSS-scale frequency rather than stretched to viewport.

This can still use `feTurbulence`, but **not as direct gray noise**. Either:

1. generate a small lit fiber texture using `feDiffuseLighting`; or
2. bake it as part of the authoring step.

### Optional Layer D — extremely broad formation variation

Prefer folding this into the surface map. If separate, it should be 300–900 px scale and <~1.5% luminance modulation.

There is deliberately **no fold overlay layer**.

---

# 5. The height-field model

## 5.1 Core principle

Construct one scalar height field:

`H(x, y)`

Then derive surface gradients/normals from it:

`dx = H(x+1,y) - H(x-1,y)`
`dy = H(x,y+1) - H(x,y-1)`

Approximate normal:

`N = normalize((-Sx * dx, -Sy * dy, 1))`

Use one fixed distant light from upper-left/front:

`L ≈ normalize((-0.45, -0.55, 0.70))`

Diffuse response:

`D = clamp(dot(N, L), 0, 1)`

Do **not** add meaningful specular.

Final surface multiplier should be compressed near 1:

`shade = 1 + gain * (D - mean(D))`

The important point is not these exact equations; it is that **all visible light/dark deformation is caused by the gradient of one continuous height field**.

That guarantees coherent lighting. It is fundamentally different from drawing shadow/highlight strokes.

## 5.2 Height field composition

Recommended conceptual field:

`H = 0.55 * H_cockle + 0.25 * H_broad + 0.20 * H_micro`

But micro fibers may remain separate if that produces cleaner assets.

### H_broad — sheet formation

- 2–4 octaves;
- characteristic wavelength ~250–700 px;
- very low amplitude;
- only prevents mathematical flatness.

### H_cockle — main visual material cue

- several overlapping band-limited noise/octave fields;
- characteristic wavelengths roughly **25–180 px**;
- weak directional bias is allowed but must not make parallel waves;
- use domain warping or mixed rotated fields to avoid obvious Perlin blobs;
- clamp slopes, not heights, so broad forms remain smooth;
- no Voronoi ridge outlines;
- no long single Bézier features.

A useful deterministic construction that does not require a noise library:

- create 20–40 seeded low-amplitude sinusoidal/Fourier modes;
- randomize angle, phase, and wavelength within selected bands;
- weight high frequencies less;
- add 1–2 warped coordinate fields;
- normalize;
- optionally blur slightly.

This produces a globally continuous field and can be made periodic/seamless exactly.

Alternatively use a small local dev-only noise library if already available, but do not add a runtime dependency.

### H_micro — optional normal perturbation

- wavelength 1–7 px;
- low amplitude;
- some anisotropic/curly fiber character;
- should not be visible as “grain dots”.

---

# 6. Concrete visual parameter envelope

These are starting constraints, not magic constants.

## 6.1 Base

- `#F2F0E9` initial.
- Target perceived screen lightness: roughly “off-white paper”, not cream.
- Final screenshot should be visibly different from Cobalt/Magenta pure white, but still bright.

## 6.2 Broad surface / cockling

- dominant feature size: 30–160 CSS px;
- secondary broader features: 180–600 px;
- local luminance modulation from surface lighting: typically within about **±2.5–4%** of base;
- rare combined extrema may approach ~5%, but no obvious gray stains.

## 6.3 Micro fiber

- 1–6 px;
- luminance modulation roughly **±0.5–1.2%**;
- anisotropy subtle;
- no repeating radial dots.

## 6.4 Lighting

- one upper-left diffuse light;
- no obvious vignette;
- specular = zero/negligible;
- no “white ridge” highlights.

## 6.5 Distribution constraints

- no identifiable ridge/valley should visually span >~40–50% of the viewport;
- no straight or smooth path should read as a crease;
- no two giant features should become “the composition”.

The material must be **statistical**, not illustrative.

---

# 7. Ink-on-paper model

Do not apply a fake newspaper effect to all typography.

## 7.1 Text

Keep:

- primary text crisp near-black;
- secondary text charcoal;
- existing typography and layout.

No text blur, transform jitter, chromatic aberration, halftone, or distressed font.

## 7.2 Heatmap cells

Keep the existing absolute magnitude mapping exactly.

For Newsprint only:

`finalCell = baseGray(intensity) × mottle(x,y)`

where mottle:

- is deterministic;
- has mean ≈ 1;
- amplitude only ~1–2.5%;
- characteristic scale ~5–24 px;
- may use a different deterministic offset per cell;
- must never reverse intensity ordering.

Strong invariant:

If `tokenA < tokenB` with a meaningful difference, the *mean luminance* of B must still be darker than A.

## 7.3 Rank bars and activity line

Optional very slight mottle/raggedness is acceptable only if it remains crisp at normal zoom.

Default choice: keep them crisp black.

The paper surface itself already provides enough material character.

---

# 8. Authoring strategy

## 8.1 First isolate the material

Before integrating into the whole page, generate a **material-only 1200×900 swatch**.

No UI.

This prevents us from misdiagnosing typography/layout as material quality.

The swatch should contain:

- 70% plain paper;
- one 25% gray test rectangle;
- one 70% black test rectangle;
- a few 1px/2px black rules;
- a small sample of normal body text.

No decorative folds.

## 8.2 Internal diagnostic stages

The coding agent may generate these during development, but they are not user-facing palette candidates:

1. substrate + cockling lighting;
2. + fiber/roughness;
3. + ink mottle test.

Use them to identify which layer causes artifacts.

## 8.3 One tuning pass

After first integrated screenshot:

- if it reads as cloudy → decrease low-frequency albedo variation, retain normal-derived shading;
- if it reads as noise → reduce micro layer;
- if it reads flat → increase **cockling slope/gain**, not add a crease;
- if it reads dirty → brighten base/reduce dark modulation;
- if it reads like marble/clouds → narrow frequency bands and reduce broad FBM dominance;
- if a visible “line” appears → remove the responsible mode/feature; do not decorate it.

---

# 9. Implementation routes, ranked

## Route 1 — recommended: deterministic bake generator in repository

Add a dev utility, e.g.:

`scripts/gennewsprint/`

Responsibilities:

- fixed seed;
- generate periodic/multiscale height field;
- derive normals/diffuse;
- emit `newsprint-surface.png`;
- optionally emit `newsprint-fiber.svg`;
- emit a diagnostic swatch;
- deterministic output hash.

No runtime JS dependency.

This is the best fit for whereToken.

### Why Go is reasonable

The repo is already Go-centric and the standard library can generate PNG. We do not need a physically exact renderer; simple height-field math is enough.

## Route 2 — dev-only Paper Design shader bake

Use Paper Design’s paper shader as a **reference/authoring tool only**, not a production dependency.

Starting conceptual parameters:

- `folds = 0`
- `roughness ≈ 0.25–0.40`
- `fiber ≈ 0.15–0.28`
- `fiberSize ≈ 0.20–0.35`
- `crumples ≈ 0.10–0.22`
- `crumpleSize ≈ 0.40–0.70`
- `drops ≈ 0–0.03`
- `contrast ≈ 0.12–0.25`
- `fade ≈ 0.10–0.30`
- fixed seed

Then bake a static texture and remove the runtime package.

If code is copied from the project rather than merely referenced, comply with Apache-2.0 attribution/NOTICE obligations. Prefer not to copy.

## Route 3 — CC0 PBR material as authoring source

Only if procedural attempts fail.

Use a CC0 paper material (e.g. ambientCG/another verified CC0 source) to obtain displacement/normal characteristics, then:

- crop/tile;
- calibrate scale;
- retint substrate;
- heavily optimize;
- bake only the useful map.

Do not ship a giant 4K/8K material set.

## Route 4 — generative image

Last resort only. Not recommended.

---

# 10. Asset and CSS architecture after the change

Target conceptual structure:

```text
internal/profilewebembed/static/assets/
  profile.css
  profile.js
  newsprint-surface.png      # baked cockling/waviness lighting
  newsprint-fiber.svg        # micro material texture (optional)
```

Remove:

```text
newsprint-folds.svg
```

Remove from CSS:

```css
--paper-crease
--paper-crease-size
body::before { background: var(--paper-crease); ... }
```

Possible final Newsprint CSS concept:

```css
:root[data-activity-palette="newsprint"] {
  --page: #f2f0e9;
  --surface: transparent;
  --paper-surface: url("./newsprint-surface.png");
  --paper-fiber: url("./newsprint-fiber.svg");
}

html,
body {
  background-color: var(--page);
}

:root[data-activity-palette="newsprint"] body {
  background-image:
    var(--paper-fiber),
    var(--paper-surface);
  background-repeat: repeat, repeat;
  /* final calibrated sizes, not viewport-stretched */
}
```

Do not blindly copy these exact sizes/colors; calibrate from screenshots once.

The key architectural rules:

- texture scale should stay roughly consistent in CSS pixels across viewport widths;
- do not `background-size: 100% 100%` the material;
- surface map must tile without obvious seam;
- header/surfaces stay transparent enough for one continuous substrate.

---

# 11. Objective acceptance criteria

The Newsprint pass is accepted only if all of these hold.

## A. Material read

At 100% browser zoom, with the heatmap temporarily hidden in a diagnostic screenshot, the page still reads as paper.

## B. No crease signatures

There must be:

- no explicit crease SVG paths;
- no long shadow/highlight lines;
- no feature that a reviewer can point to as “the vertical fold” or “the diagonal fold”.

## C. Multiscale behavior

A crop should show:

- broad surface waviness;
- medium cockling;
- fine fiber/roughness.

Not just one noise frequency.

## D. Physically coherent shading

All meso/macro shading comes from the same height/normal/light model or equivalent bake.

No randomly painted highlight unrelated to geometry.

## E. Matte substrate

No shiny/specular white streaks.

## F. Color

Paper is visibly warmer/grayer than `#FFF` but not yellow/sepia/dirty.

## G. No obvious repetition

At 1440×900 and 390×844 there is no obvious tile seam or recognizable repeated blob.

## H. Content legibility

Text contrast and thin rules remain excellent.

## I. Data semantics unchanged

Token magnitude mapping is bit-for-bit/behaviorally unchanged.

## J. Performance

No permanent animation, no full-page runtime WebGL, no high-cost filter on every frame.

## K. Three-browser stability

Existing Chromium/Firefox/WebKit profile E2E remains green.

---

# 12. Screenshot QA protocol

Required:

1. 1440×900 full Newsprint page.
2. 390×844 mobile Newsprint page.
3. 1200×900 material-only diagnostic swatch.
4. 400×400 100%-scale crop of plain paper.
5. 400×400 crop including heatmap cells.

Also compare against baseline commit `7d9a162`.

Specific review questions:

- Does the baseline obviously show two crease paths? Yes — expected.
- Does the new version remove identifiable paths entirely?
- If viewed for only 1 second, does the new background say “paper” rather than “noise”?
- If blurred mentally/squinted, are there irregular broad low-amplitude variations rather than two lines?
- Does the close crop show fine fibers without dots/static?
- Is the page brighter/cleaner than aged parchment?
- Does the surface remain continuous behind header and content?

---

# 13. Testing

Keep existing tests.

Add targeted tests for architecture/semantics:

- `newsprint-folds.svg` no longer part of canonical bundle.
- no `--paper-crease` reference.
- baked asset exists in canonical source.
- generated production/demo include correct asset.
- deterministic generator produces same hash from same seed.
- generated asset dimensions are fixed and documented.
- asset file size stays under a reasonable budget chosen after generation.
- no external asset URL.
- Newsprint theme persists.
- absolute heat intensity formula unchanged.
- Cobalt/Magenta computed theme values unchanged.
- production validation.
- full browser E2E.
- `go test ./...`
- `go vet ./...`
- `git diff --check`.

---

# 14. Performance/file-size target

Do not let “PBR” become a giant web asset.

Initial budget:

- `newsprint-surface.png`: target < 250 KB, hard review threshold 450 KB;
- `newsprint-fiber.svg`: target < 20 KB;
- no runtime shader library;
- no 4K/8K source texture in production bundle.

If PNG is larger because of high-frequency noise, that is evidence that the high-frequency component belongs in the tiny fiber layer rather than the baked surface PNG.

---

# 15. What “success” looks like

The final page should not call attention to any particular wrinkle.

Instead, when switching from Cobalt/Magenta to Newsprint:

- the pure white digital canvas becomes slightly duller/more fibrous;
- flatness disappears;
- broad, irregular paper undulations are perceptible;
- gray and black data marks feel absorbed into a matte substrate;
- the whole visual language becomes tactile and printed;
- nothing resembles an “old paper” effect;
- nothing resembles an SVG crease demo.

That is the target.

---

# 16. Research references used for this design

1. Paper Design — Paper Texture shader: separates roughness, fiber, crumples, folds, fade, drops and uses a surface-normal lighting model.
2. MDN — SVG `feDiffuseLighting`: bump-map-based diffuse lighting from surface geometry.
3. MDN — SVG `feTurbulence` / `feDisplacementMap`: procedural texture and displacement primitives with broad browser support.
4. UPM News C technical data: matte newsprint, brightness/Lab/roughness values.
5. Paper surface topography literature: roughness/cockling/waviness across broad spatial wavelengths.
6. Conservation definitions of cockling: planar distortion/rippling can occur without creases.
7. Print-mottle research: stochastic cloudiness/graininess tied to surface roughness and uneven ink setting/absorption.
8. Poly Haven PBR workflow: separates albedo, displacement/height, normal and roughness, reinforcing the principle that material shape and color are different signals.

The specification above is an adaptation for a web UI, not an attempt to reproduce laboratory measurements literally.
