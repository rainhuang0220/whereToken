# Public profile visual research

Visual direction: quiet, editorial, instrument-like, personal, data-first, bespoke, and GitHub-adjacent. This note is visual only; it does not change token math, data semantics, source coverage, or interaction contracts.

## Current-state audit

- The baseline preview was a useful 800×248 footprint, but indigo totals, a violet heat scale, cyan peak outline, 12px outer radius, and a colored “Explore profile” CTA made it read as a product card rather than a personal activity instrument.
- The live page repeats containers: glow background, 14px shadowed trend/heat cards, pill freshness and coverage chips, three pill-style segmented controls, rounded rank cards, and pill bars. At 720px the masthead wraps into two prominent control rows before the data.
- The 44–72px hero number is oversized relative to the 15px section titles. The large empty field to its right makes the page feel like a dashboard with a missing second KPI.
- The live 11px/3px activity grid is close to GitHub's density. The preview's 9px/2px grid uses only about 583px of its 800px canvas and feels miniaturized.
- There are no committed public-profile PNG screenshots; the two generated SVGs and the live `public-profile/` bundle were rendered and inspected in light/dark desktop plus 720px narrow view on 2026-09-15.

## Primary-source findings

- GitHub describes the contribution calendar as the year-scale visual overview of profile activity; it is supporting profile identity, not a standalone dashboard. Keep one dominant calendar and make secondary statistics subordinate. [GitHub Docs — Contributions on your profile](https://docs.github.com/en/account-and-profile/concepts/contributions-on-your-profile)
- GitHub's contribution API exposes explicit days, weeks, month spans, relative contribution levels, and an ordered color list. Preserve explicit month/day structure and one sequential scale; do not add an unrelated peak hue. [GitHub GraphQL — `ContributionCalendar`](https://docs.github.com/en/graphql/reference/objects#contributioncalendar)
- A current logged-out GitHub profile renders the calendar as an accessible grid, with 10px cells, 3px border spacing, a 13px month-label row, a 28px weekday-label gutter, horizontal overflow, per-day focus, and “Less/More” legend. Use this as a density/accessibility reference, not as a skin to copy. [GitHub profile contribution view](https://github.com/torvalds?tab=contributions)
- Primer's semantic neutrals are the right GitHub-adjacent foundation: light default `#ffffff`, inset `#f6f8fa`, text `#1f2328`, muted `#59636e`, border `#d1d9e0`; dark default `#0d1117`, inset `#010409`, text `#f0f6fc`, muted `#9198a1`, border `#3d444d`. Accent blue belongs to links/focus, not headline data. [Primer color primitives](https://primer.style/product/primitives/color/)
- Primer's compact type ramp is 12px caption/body-small, 14px body, 16px title-small, 20px title-medium, 32px title-large, and 40px display; it explicitly advises against using color as the primary hierarchy device. [Primer typography primitives](https://primer.style/product/primitives/typography/), [Primer typography guidance](https://primer.style/product/getting-started/foundations/typography/)
- Primer's standard radius is 6px, large radius 12px, and borders are 1px; its relevant breakpoints are 544px, 768px, and 1012px. A 6px frame and hairline rules are more GitHub-adjacent than 12–14px cards everywhere. [Primer size primitives](https://primer.style/product/primitives/size/)
- Microsoft's first-party visualization guidance says data ink should remain central and specifically warns against drop shadows, heavy outlines, and decorative gradients. This directly supports deleting the glow, card shadows, and peak halo. [Microsoft data visualization guidelines](https://learn.microsoft.com/en-us/office/dev/add-ins/design/data-visualization-guidelines)
- The UK Office for National Statistics says not to add chart borders or backgrounds that distract from the data, and recommends direct labels where practical. Use one section rule and direct values instead of nested chart cards and legends. [ONS chart elements](https://service-manual.ons.gov.uk/data-visualisation/build-specifications/chart-elements)
- Urban Institute's original visualization guide uses left-aligned titles, a defined hierarchy, compact 11–20px chart text, and a 760px editorial chart width. This supports a narrower reading column and sentence-case labels. [Urban Institute visualization style guide](https://urbaninstitute.github.io/graphics-styleguide/)
- Mike Bostock's original D3 calendar view and Observable Plot calendar demonstrate that the week/day matrix itself can carry the visual identity without ornamental framing. [D3 Calendar](https://observablehq.com/@d3/calendar), [Observable Plot Calendar](https://observablehq.com/@observablehq/plot-calendar)

## Executable visual specification

### GitHub preview SVG

- Keep `800×204` for README compatibility. Use one flat canvas with a 1px border and 6px radius; no shadow, glow, gradient, inner cards, or CTA block.
- Use 30px outer margins. Header baseline at `y=27`, hairline at `y=43.5`; brand at `x=30` and snapshot date/provenance at `x=770`, right-aligned.
- Summary row: total at `x=30, y=79`, 30px/600/tabular with the unit inline; the `53 weeks` label is right-aligned at `x=770, y=82`. No KPI boxes.
- Activity wall begins at `x=30, y=96`. Use 53 columns, seven rows, 12px cells, 2px gaps, and 2px radius. This occupies `740×96px` and ends at `x=770, y=192`.
- Remove the cyan peak stroke. The maximum day is simply level 5; focus is irrelevant in static SVG. Put `DEMO DATA` or partial status in header text so provenance remains visible after deleting the footer badge.
- Remove “Explore profile →”; the README image/link already supplies navigation. Keep the accessible `<title>`, `<desc>`, snapshot metadata, and raw-data privacy wording.

### Live public profile

- Masthead: 54px minimum height, 1px bottom rule, no chips. Align an inner `max-width: 1120px` container to the main column. Show brand left; date, coverage text, theme text control, and GitHub link right. At ≤720px allow two quiet rows without framed pills.
- Main: `max-width: 1120px; padding: 30px 24px 76px`; use 16px side padding below 720px. Keep prose such as “About” at `max-width: 68ch`.
- Hero: cap the total at 42px/600 (34px below 720px). Put the unit on the same typographic line. Render requests, active days, and current streak as a ruled definition row that collapses to two columns below 720px. Do not introduce KPI cards.
- Section rhythm: 48px between major sections, 16px/600 headings, 14px body, 12px captions. Use sentence case; reserve monospace/tabular numerals for values, dates, and readouts.
- Controls: remove the surrounding `.seg` capsule. Use plain text tabs with 8–12px inline spacing and a 2px bottom rule for selection. A single selected underline is enough; do not tint every selected background.
- Trend and heatmap: place them in one Activity section separated by a 1px rule, without cards, shadows, or independent backgrounds. Trend height 72px, line 1.5px, no area fill. Use 15px heat cells/3px gap/2px radius at desktop, 13px at tablet, and 14px in the mobile scroller; preserve the weekday gutter, month labels, horizontal scroll, tooltips, and keyboard focus.
- Breakdown: use a ruled editorial table/list (`32px minmax(140px,1fr) auto`) rather than rounded row cards. Use a 2–3px square-ended bar or right-aligned percentage; selected state gets a 2px left rule plus weight change, not a tinted card.
- Coverage: retain disclosure behavior, but render it as a ruled section. Reserve status colors for status semantics; do not use the activity hue for “partial”.

### Color tokens

Use Primer neutrals unchanged for adjacency; use a distinct, low-chroma moss scale only for activity data.

| Role | Light | Dark |
| --- | --- | --- |
| Page / surface | `#ffffff` | `#0d1117` |
| Inset / future | `#f6f8fa` | `#010409` |
| Border / rule | `#d1d9e0` | `#3d444d` |
| Text | `#1f2328` | `#f0f6fc` |
| Muted text | `#59636e` | `#9198a1` |
| Link / focus only | `#0969da` | `#4493f8` |
| Empty | `#eff2f5` | `#151b23` |
| Heat 1 | `#d9eadc` | `#183023` |
| Heat 2 | `#acd3b4` | `#23482f` |
| Heat 3 | `#78b487` | `#326440` |
| Heat 4 | `#43875b` | `#4e865e` |
| Heat 5 / maximum | `#285e3d` | `#79ad86` |

Delete `--secondary`, `--peak`, and `--glow`. Keep unknown distinct from empty with a neutral hatch; keep future distinct with inset color. Never encode peak with a second hue.

## Visual QA risks

- Render the SVG through GitHub/Camo in light and dark; test 800px native and common README downscales. Check 5–6 character totals, em dashes, long agent labels, partial state, and `DEMO DATA` for clipping.
- Test live at 1280, 1012, 768, 720, 544, and 390px. The document must not overflow; only the calendar scroller may overflow horizontally, and it must start at recent weeks.
- Test heat levels in grayscale and common color-vision simulations. Color is not enough for unknown/future/selected/peak states; retain text/tooltips, hatch, and focus/selection outlines.
- Keep 2px visible keyboard focus on cells, tabs, disclosure, theme control, and links. Removing pills must not remove affordance or target size.
- Check dark-mode rules and empty cells on OLED-like black: `#010409` inset against `#0d1117` can disappear unless the `#3d444d` structural rule remains where needed.
- Existing preview tests assert `800×248`, current background colors, and the “Explore profile” CTA. A redesign should intentionally update only the visual assertions while preserving snapshot ID, privacy copy, provenance, and light/dark structural parity.
