# Public profile design decision

## Direction

Treat the public profile as a personal coding-activity sheet: quiet, editorial, instrument-like, and GitHub-adjacent. The 53-week activity wall is the dominant visual; typography, alignment, proportion, and hairline rules provide hierarchy. The palette is warm neutral with a monotonic oxide/ember heat scale. There are no gradients, glows, glass effects, floating cards, decorative shadows, or cyan peak accents.

## Preview

Keep an 800px canvas but reduce it to 204px high. Use 30px side margins and a 53×7 wall made from 12px cells with 2px gaps: 740×96px. Show only `whereToken`, a short factual descriptor, the update date, one 28–32px token total, the wall, and a quiet demo marker when provenance requires it. The whole README image is the link; the SVG has no CTA or secondary KPI row.

## Interactive profile

Use one 1120px reading frame. Put a restrained masthead above a compact total, text-tab controls, and the full activity wall. Desktop cells are 15px (13px at tablet widths); mobile cells are 14px inside a horizontal scroller positioned at the newest weeks. Keep the trend as an unfilled line strip below the wall. Breakdown is a ruled ranked list with square-ended proportional bars. Coverage and methodology share one collapsed technical-details section at the bottom.

## Interaction and publication

README SVGs are static: GitHub/Camo renders the asset as one `<img>`, so per-cell hover is intentionally limited to the interactive page. Tooltip state tracks the latest trigger, cancels pending hide/position work, closes on rerender, and is clamped to the viewport. `snapshot_id` keeps its data-only meaning; the bundle adds a content-derived `asset_revision`, and README URLs use both values so renderer-only changes invalidate Camo without falsifying the snapshot identity.
