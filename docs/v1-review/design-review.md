# whereToken v1 Design & Product Review

**Reviewer:** Reviewer B (Independent Senior Product/Design Director)  
**Repository:** `/workspace`  
**Revision:** `de3dfb4` on branch `cursor/v1-maturity-review-27f1`  
**Review Date:** 2026-09-16  
**Review Mode:** Read-only inspection · No code modifications

---

## Executive Summary

**Does this feel like a real product?** Yes, conditionally. The kiln/furnace metaphor is strong and unique. The public profile and dashboard demonstrate clear product thinking. However, Chinese-first UI creates friction for English-speaking users evaluating the product, and some presentation choices feel like they serve internal engineering constraints rather than external clarity.

**Does it have unique identity?** Yes. The "窑" (kiln/furnace) mascot, ember palette, token-as-firing metaphor, and material design vocabulary ("forge", "brick", "glaze", "mortar") establish a coherent and memorable identity. This is not generic SaaS.

**Where does it feel AI-generated or template-like?**
- Landing page card grid ("What it counts") uses safe, expected feature-card language
- Some dashboard label hierarchies (tool/vendor/model breadcrumbs) read as data-structure labels rather than user-facing copy
- The "theme gallery" keyboard visualization is clever but feels like placeholder spectacle

**What should be removed?**
- The elaborate animated keyboard mock in the theme gallery: it's impressive but adds no product value
- Redundant "演示数据" / "demo data" labels that appear multiple times
- The decorative `.forge::before` scanline effect: it's subtle but purposeless

**What should become stronger?**
- **English-first or bilingual UI** for global adoption
- **Newsprint material authenticity** (see dedicated section)
- **Public profile identity**: currently GitHub-styled; needs whereToken personality
- **Landing page hero clarity**: "fires them into one kiln" is poetic but ambiguous

---

## 1. GitHub-Facing Experience

### 1.1 README.md

**Observed (Desktop, 1280px):**
- Clean, well-structured Markdown with proper hierarchy
- Logo (96px) + centered H1 creates strong first impression
- Badge row (status, release, CI, license, Go version) establishes legitimacy
- Feature screenshot (`dash-newspaper.jpg`) immediately shows the product
- Chinese UI in screenshot may deter non-Chinese readers before they read feature list

**Strengths:**
- Comprehensive install instructions (macOS/Linux/Windows + Homebrew + go install)
- "Missing usage is shown as unavailable, never as a fake zero" is excellent honesty-in-design messaging
- Privacy section is transparent and detailed
- Supported agents list is impressive (15 tools)
- Documentation is well-linked and organized

**Weaknesses:**
- Screenshot is labeled "墨 is the monochrome, newspaper-style theme" but image shows Chinese interface—may confuse English readers about whether product supports English
- The "窑" mascot is introduced late (Dashboard section) without explanation of metaphor
- Some feature descriptions are vague: "deterministic usage portrait (用户画像)" needs explanation before Chinese term
- "whereToken card" command is described as legacy but no migration guidance

**Identity verdict:** Professional, technical, honest. Not AI-generated. Reads like documentation written by engineers who care about accuracy. The kiln metaphor is present but understated.

---

## 2. Landing Page (site/index.html)

**Inspected URL:** `localhost:8080/site/`  
**Viewport:** 1280×800 desktop, then scrolled

### 2.1 Visual Design

**Observed:**
- Dark background (`--bg: #0c0a09`, near-black)
- Ember orange (`--ember: #f97316`) as primary accent
- Gold (`--gold: #fbbf24`) for links and secondary highlights
- Monospace font for code/UI elements, sans-serif for body
- Clean, generous white space
- Screenshot of kiln dashboard prominently displayed

**Strengths:**
- Color palette is distinctive and consistent with kiln/furnace theme
- Dark mode reduces eye strain for developer audience
- Typography hierarchy is clear
- CTAs are prominent ("Open Web App", "Demo", "Download CLI")

**Weaknesses:**
- Hero text "Your agents keep the receipts. whereToken fires them into one kiln. 一窑烧出真账。" is clever but requires mental translation
- "fires them into one kiln" is metaphorical—some users won't immediately grasp "kiln = unified view"
- Chip grid ("Supported ledgers") has "Cursor · account API" in different style but distinction isn't explained in visual design
- Privacy section uses ✕ and ✓ checkmarks which work, but bullet styling could be refined

**Template risk:** The card grid ("What it counts") has conventional feature-card layout. Cards are saved by strong copy ("Cache honesty", "Streaks, not guilt") but visual treatment is standard.

### 2.2 Content & Messaging

**Hero:**
> "Your agents keep the receipts. whereToken fires them into one kiln."

**Analysis:** Poetic, memorable, but requires inference. "Kiln" metaphor is not self-evident to users unfamiliar with pottery/ceramics. Consider: does a developer immediately understand "one kiln" means "unified analytics"?

**Feature Cards:**
- "Every tool, one table" — clear
- "Cache honesty" — excellent, differentiating
- "Price from public lists" — strong transparency message
- "Streaks, not guilt" — human, empathetic tone; best card

**Verdict:** Copy has personality and product conviction. Not AI-generated filler. Some metaphors need one more iteration for clarity.

### 2.3 Accessibility & Responsiveness

**Not tested at mobile width in browser**, but CSS has `@media (max-width: 900px)` breakpoints for responsive layout. Code review suggests proper responsive treatment.

---

## 3. Public Profile & Newsprint

**Inspected URL:** `localhost:8080/public-profile/`  
**Viewports:** Desktop (1280×800), Mobile (400×924 via DevTools)  
**Theme tested:** Newsprint

### 3.1 Public Profile Structure

**Observed:**
- Clean header with "whereToken · my coding-agent token usage"
- Theme switcher (System / Light / Dark) + palette switcher (Cobalt / Magenta / Newsprint)
- Large hero number (10.76B tokens)
- GitHub-style contribution heatmap (53 weeks, Mon/Wed/Fri labels)
- 30-day activity line chart
- Breakdown tables (Agents / Providers)
- "Technical details" disclosure with coverage table

**Strengths:**
- Layout is logical and familiar (GitHub contribution graph pattern)
- Data hierarchy is clear: hero number → heatmap → trend → breakdown
- Theme switcher is prominent and functional
- Mobile responsive layout works well (tested via DevTools at 400px)

**Weaknesses:**
- **Visual identity is too GitHub-generic.** The heatmap, color scale, and overall structure closely mirror GitHub's contribution graph. While familiar, it doesn't express whereToken's unique kiln/furnace identity.
- Typography is clean but conservative (system fonts, standard weights)
- No "窑" mascot or kiln visual language in public profile
- Theme switcher buttons are plain text with underline-on-active; could be more distinctive

**Identity gap:** Public profile feels like a GitHub stats page, not a whereToken artifact. The landing page and dashboard have strong kiln branding; the public profile doesn't carry it forward.

### 3.2 Newsprint Theme: Material Review

**Inspected:** Newsprint palette active, zoomed to 100%, scrolled page, checked mobile

**Background implementation:**
```css
:root[data-activity-palette="newsprint"] body {
  background-image: url("./newsprint-surface.jpg");
  background-repeat: repeat;
  background-size: 560px auto;
  background-position: 0 0;
  background-blend-mode: normal;
}
```

**Asset:** `newsprint-surface.jpg` (151 KB, 1280×798 JPEG, referenced as CC0-derived from ambientCG Paper001)

**Observed material quality:**

#### Does it feel like real paper?
**Partially.** The background has visible texture and subtle tonal variation that reads as paper rather than pure white or flat color. At 100% zoom on a bright display, the page has a warm, slightly fibrous quality distinct from the Cobalt/Magenta pure-white themes.

#### Material authenticity assessment

**Strengths:**
- The texture is **continuous and multiscale**, not a single noise layer
- No obvious **crease paths or illustrated folds** (the rejected v2 SVG path model is gone)
- Tiling is **seamless**; no visible seam at 560px repeat
- Base color (`--page: #f7f6f1`) is appropriately off-white and warmer than screen white
- Text contrast remains excellent (--text: #161616 on warm off-white)
- Texture does not interfere with heatmap legibility
- Mobile: texture scales proportionally, no breakage

**Weaknesses & "Real Material" Test:**

1. **Texture reads as scan, not as paper surface with light response.**
   - The current material is a **uniform tiled photograph**, not a height-field with directional lighting
   - Paper Design spec emphasized: "surface = several independent frequency families... use a surface-normal/light model"
   - Current result: tonal variation is **baked into pixels** rather than derived from geometry
   - **Verdict:** This is a scanned paper photo, not a simulated paper material

2. **Cockling/waviness is subtle to invisible.**
   - The design spec called for "broad irregular hills and valleys" at ~25–180px scale
   - Observed: texture has fine fiber-scale variation but **no obvious meso-scale undulation**
   - The page feels flat, not gently rippled
   - **Verdict:** Missing the key "cockled paper" cue that would distinguish this from a simple paper scan filter

3. **No perceptible ink-on-paper effect.**
   - Design spec mentioned optional mottle on heatmap cells
   - Observed: heatmap cells are **flat gray**, no absorption texture
   - This is fine—data clarity is more important than decoration—but the material system doesn't extend to ink

4. **Texture scale may be too small.**
   - At 560px tile size on a 1280px viewport, the pattern repeats ~2.3 times horizontally
   - On large displays (1440px+), repetition becomes more noticeable
   - Recommendation: consider 1024px or 1280px tile for desktop (with appropriate file-size optimization)

5. **JPEG artifacts at close inspection.**
   - 151 KB JPEG shows mild compression artifacts when zoomed
   - For a "material" meant to convey quality, consider PNG or higher-quality JPEG
   - File size is reasonable; quality could increase without major cost

**Newsprint Design Spec Compliance:**

From `docs/wheretoken_newsprint_material_design_v3.md`:
- ❌ "Construct one scalar height field H(x, y)" → not implemented; using photo scan instead
- ✅ "no explicit crease SVG paths" → confirmed removed
- ⚠️  "cockling/waviness 25–180 px" → not visibly present
- ✅ "micro fiber 1–7 px" → present in scan
- ❌ "diffuse lighting from surface gradients" → not implemented (baked scan has no dynamic lighting)
- ✅ "seamless/tile-safe" → confirmed seamless
- ✅ "`--newsprint-base: #F2F0E9` (approx)" → actual `#f7f6f1` is close and appropriate
- ✅ "no line-shaped feature" → confirmed, no crease lines

**Acceptance Criteria Review (from spec §11):**
- ✅ A. "page still reads as paper" → yes, but subtly
- ✅ B. "no explicit crease SVG paths" → confirmed
- ⚠️  C. "multiscale behavior" → fiber present, cockling absent
- ❌ D. "physically coherent shading" → not applicable (photo scan, no lighting model)
- ✅ E. "matte substrate" → yes, no specular
- ✅ F. "color visibly warmer than #FFF" → yes
- ✅ G. "no obvious repetition at 1440×900" → acceptable (marginal at 1280)
- ✅ H. "content legibility" → excellent
- ✅ I. "data semantics unchanged" → confirmed
- ✅ J. "performance" → no animation, no runtime cost

**Overall Newsprint Verdict:**

The current Newsprint implementation is a **pragmatic, production-ready compromise** that **successfully removes the artificial crease-path model** (the primary design failure from v2) and delivers a **functional, legible, non-distracting paper background**. It does NOT implement the full height-field PBR-lite material system described in the design spec.

**What it is:** A high-quality CC0 paper scan, retinted, tiled, and applied as a static background.  
**What it isn't:** A procedurally lit, multiscale, cockled paper material with surface normals and diffuse shading.

**Is this acceptable for v1?** **Yes, with caveats.**
- It's honest: real scanned paper, not fake illustrated texture
- It's restrained: doesn't interfere with data
- It's on-brand: editorial/print feeling aligns with "newsprint" name
- It's better than v2's crease paths by a wide margin

**Is this the design spec's vision?** **No.**
- Missing: height-field cockling, directional lighting, multiscale waviness
- Present: seamless tiling, appropriate color, good legibility

**Recommendation for future iteration (not v1 blocker):**
1. Generate a true height-field material per the spec (cockling + fiber)
2. Bake diffuse lighting from normals (not runtime shader—still static PNG)
3. Use 1280px or 1536px tile for large displays
4. Consider WebP or high-quality JPEG (200–250 KB acceptable for premium feel)
5. Keep current scan as "newsprint-scan" fallback theme for low-bandwidth

**Does the current result feel AI-generated?** No. It feels like a careful, conservative product decision: use real material (CC0 scan) rather than risk generating an artificial texture. This is **design humility**, not laziness. The missing cockling is the only aesthetic gap.

---

## 4. Dashboard (Local/Demo) — UPDATED WITH ACTUAL RENDERING

**Inspected URL:** `localhost:5173/whereToken/demo/` via Vite dev server (`VITE_DEMO=1`)  
**Viewports:** Desktop (~1280px), Mobile responsive mode (~860px)

## Dashboard Update: Actual Rendering Inspected

**Previously:** Section 4 stated "Attempted inspection... failed to load" with caveats throughout.

**Now inspected:** Working dashboard at `localhost:5173/whereToken/demo/` via Vite dev server with `VITE_DEMO=1`.

### Desktop Dashboard (1280px viewport, scrolled)

**Header & Identity:**
- 窑 (kiln) mascot in yellow/gold displayed prominently top-left
- "whereToken" in large gold typography with subtitle "本机 token 窑" (local token kiln)
- Date/time stamp top-right: "2026/9/4 84:59:10 · 演示数据"
- "主题" (Themes) button top-right

**Time Period Controls:**
- Four yellow/gold buttons: "今日" (Today), "7 天" (7 days), "30 天" (30 days), "全部" (All)
- Clear visual hierarchy with active state

**Axis Filter ("Damper"):**
- Horizontal scrolling agent list: "合计" (Total), "工具" (Tools), followed by agent names (Claude Code, Kimi Code, Codex, Cursor, ZCode, Gemini CLI, Grok, OpenCode, etc.)
- Gold text for active filter, muted for inactive
- Clean horizontal layout with good spacing

**Kiln Wall (Brick Calendar):**
- Dominant visual element: 7-row × 53-column grid of "bricks"
- Each brick is a small square (13px) representing one day
- Color-coded by intensity: dark gray (empty) → yellow/orange/red (increasing token usage)
- Right side shows concentrated orange/yellow "hot" period (recent activity spike)
- Border: thin copper-colored frame
- Legend at bottom-right showing intensity scale
- **Dominance assessment:** The wall occupies ~60% of viewport width, clearly the primary visual

**Right Column Stats:**
- Large gold numbers: "16.78 M" (tokens)
- Streaks: "27天" displayed twice (current and longest)
- Clean typography, excellent hierarchy

**2×5 KPI Grid:**
- **Top row (5 cells):**
  1. "总用量" (Total): 352.20 M (large gold)
  2. "命中率" (Hit rate): 57.5% (gold)
  3. "最长连烧" (Longest streak): 27 天
  4. "当日用量" (Today): 8.14 M
  5. "估价" (Estimate): $500.08 + note "价格按需查询 wheretoken pricing"
- **Bottom row (5 cells):**
  1. "当前连烧" (Current streak): 27 天
  2. "请求" (Requests): 1,024
  3. "用户回答" (User responses): 439
  4. "峰日最高" (Peak day): 16.78 M
  5. "排名" (Rank): "持续高负载" (sustained high load) + note

**Density verdict:** The 2×5 grid is **information-dense but not overwhelming**. Each cell has clear label + large value + optional subtitle. The grid uses visual hierarchy (font size, color) effectively to make scanning easy despite density.

**Breakdown Tables:**
- Two side-by-side tables: "按工具" (By Tool) and "按厂家" (By Provider)
- Chinese column headers: "名称" (Name), "未命中" (Miss), "缓存读" (Cache Read), "缓存写" (Cache Write), "输出" (Output), etc.
- Orange-tinted percentages (56.2%, 59.1%, 58.0%) for hit rates
- Model-level drill-down tables below ("按模型" / "按工作区")
- Dense tabular data with many columns, requires horizontal scroll on smaller viewports

### Theme Gallery ("釉" - Glaze Hall)

**Inspected at:** `localhost:5173/whereToken/demo/themes`

**Layout:**
- Black background
- Large gold "釉" (glaze) character as page title
- Subtitle: "点一块看整页。应用才带走。" (Click a tile to see full page. Apply to take it away.)
- 4×2 grid of theme cards

**Theme Count:** **8 themes total**
1. 窑 (kiln) - black/orange/gold ember theme (default)
2. 苍 (pale/ash) - light green/teal theme
3. 瓷 (porcelain) - clean white/blue-gray theme
4. 绦 (braid/magenta) - dark purple/magenta theme
5. 昼 (day) - bright blue theme
6. 墨 (ink) - grayscale/black theme
7. 漫 (diffuse) - warm beige/orange theme
8. 端 (terminal) - dark green/yellow monochrome theme

**Card Design:**
- Large Chinese character (emoji-sized) as theme mark/icon
- 2-3 lines of Chinese descriptive text (theme philosophy)
- Small color ramp preview at bottom (5 brick-colored squares showing palette)
- Cards have subtle border, clean typography

**Expanded View (Keyboard Mock):**

Clicked on "窑" (kiln) theme → full-screen expansion with **animated keyboard visualization**.

**Observed:**
- Header: "窑" character + description + "whereToken 248.60M" + controls
- **Massive keyboard layout** occupying 70% of viewport:
  - Full QWERTY layout with function row (F1-F12, Esc, Delete, etc.)
  - Color-coded by zone:
    - Orange keys: function keys, modifiers (Ctrl, Shift, Enter, Caps, etc.)
    - Brown keys: alpha keys (QWERTY letters)
    - Different orange shade: space bar
  - Each key labeled with key name
  - Clean borders between keys
  - Responsive to theme palette (kiln = orange/brown gradients)
- **Keyboard is interactive** (keys respond to mouse hover/clicks with color changes)
- Bottom buttons: "返回釉厅" (Return to gallery) | "应用" (Apply, in gold)

**Keyboard mock verdict:**
- **Technically impressive:** Clean rendering, responsive interactions, theme-colored zones
- **Functionally questionable:** Does not help user preview how the theme affects **actual dashboard UI** (KPI numbers, brick wall, tables)
- **What's missing:** A miniature dashboard preview showing kiln wall + KPI row in theme colors would be far more useful than a full keyboard
- **Engineering showcase vs. product utility:** This feels like a demo of animation capability rather than a decision-making tool

### Mobile Dashboard (Mobile responsive mode, ~860px initially)

**Observed:**
- Header wraps: mascot + title + buttons stack vertically or wrap
- Time period buttons remain horizontal (4 buttons fit comfortably)
- Agent filter (damper) becomes horizontal scroll as expected
- Kiln wall remains visible and proportional
- KPI grid: **Observed to stack differently** (CSS suggests 3-column at <900px, then block layout at <600px)
- Breakdown tables become vertical scrolling
- Overall: **Responsive implementation appears functional**

### Typography Observed (Desktop)

**Display numbers:**
- Large gold values: clamp-based scaling (352.20 M, 16.78 M)
- Font appears to be custom/display weight (bold, wide letterforms)
- Excellent contrast against black background

**UI text:**
- Chinese labels: clean sans-serif (likely "Chiron Hei HK" or similar)
- Consistent hierarchy: labels in muted color, values in gold/white
- Monospace for code/technical elements

**Table text:**
- Small but readable
- Percentage values in orange-tinted color provide visual cue for performance

### Scanline Decoration

**Looked for `.forge::before` scanline effect described in CSS.**

**Verdict:** **Not perceptible in rendered dashboard.** The CSS includes a `repeating-linear-gradient` scanline overlay at 14% opacity, but against the black background with dense content, it is effectively invisible. If present, it does not distract. The earlier recommendation to remove it stands, but its visual impact is negligible.

### Controls & Interactions

**Refresh button:** Present and functional (says "刷新" when not loading)
**Theme button:** Navigates to gallery smoothly
**Period buttons:** Clear active state (gold fill)
**Tables:** Hover states on rows (background darkens slightly)
**Overall:** Controls are well-designed, clear affordances, good hover states

### Chinese-First Language Impact

**Observed throughout:**
- All UI labels, button text, table headers, and explanatory notes are in Chinese
- No English fallback visible
- Even technical terms use Chinese: "未命中" (miss), "缓存读" (cache read), "输出" (output)
- Theme gallery descriptions are poetic Chinese phrases about each theme's character

**Verdict:** This creates a **significant adoption barrier** for non-Chinese developers evaluating the product. While the visual design and data density are excellent, the language-first choice limits international adoption potential.

### Generic Dashboard Risk Assessment

**After full inspection:**

**NOT generic.** The dashboard has strong custom identity:
- Kiln/furnace metaphor is fully realized (brick wall, ember colors, 窑 mascot, "烧" firing terminology)
- Custom color palette (ember gradients, copper accents, mortar backgrounds)
- Unique visualizations (brick calendar wall, not standard line charts)
- Material vocabulary throughout ("forge", "damper", "glaze hall")

**The only conventional elements:**
- 2×5 KPI grid layout (but content and styling are custom)
- Side-by-side breakdown tables (standard data table pattern)

**Template risk: Low.** This is not a dashboard template. The kiln theme and brick wall visualization are unique to whereToken.

### Visual Identity Coherence

**Across README / Landing Page / Public Profile / Dashboard:**

**Dashboard identity: STRONG**
- Kiln metaphor dominates
- 窑 mascot visible and functional
- Ember color palette consistent
- Material vocabulary present ("forge", brick wall, etc.)

**Coherence verdict:**
- **Dashboard ↔ Landing page:** Strong coherence (both use dark ember palette, kiln metaphor)
- **Dashboard ↔ Public profile:** **Identity gap** (profile is GitHub-styled light mode, lacks kiln branding)
- **Dashboard ↔ README:** Moderate (README mentions kiln/mascot but doesn't visually emphasize it as strongly as dashboard)

**The public profile remains the outlier** – it needs kiln identity to match the strong branding of the dashboard and landing page.

---

## Revised Conclusions Based on Actual Dashboard Rendering

### What was confirmed:

1. ✅ **Unique kiln/furnace identity is strong** – brick wall, ember palette, 窑 mascot, material vocabulary all present and coherent
2. ✅ **2×5 KPI density is appropriate** – information-rich but hierarchical, not overwhelming
3. ✅ **Chinese-first UI is pervasive** – every label, button, and description is Chinese
4. ✅ **8 themes with distinct palettes** – each has custom colors and philosophical description
5. ✅ **Animated keyboard mock in theme gallery is impressive but non-essential** – engineering showcase, low product utility
6. ✅ **Scanline decoration is imperceptible** – recommendation to remove stands, but no visual harm
7. ✅ **Generic dashboard risk is low** – custom visualizations and strong brand identity

### What changes:

1. **Kiln wall dominance is confirmed appropriate** – it's the primary visual but doesn't crowd out data
2. **Dashboard is production-quality** – no placeholder UI, no broken states observed in demo
3. **Controls are well-designed** – buttons, filters, and tables have good affordances
4. **Responsive design appears functional** – layout adapts at expected breakpoints

### What remains true from initial review:

1. **Keyboard mock should be replaced** with dashboard preview in theme gallery
2. **Chinese-first UI limits adoption** – English or bilingual is critical for v1
3. **Public profile identity gap** – needs kiln branding to match dashboard
4. **Theme count (8) is appropriate** – not too few, not overwhelming

### Updated Section 4 Recommendations:

**Keep:**
- Kiln wall visualization (dominant but appropriate)
- 2×5 KPI grid density (hierarchical and scannable)
- 窑 mascot and ember palette
- Theme gallery structure (8 themes is good)

**Remove:**
- Animated keyboard mock (replace with dashboard preview showing KPI + brick colors in theme)

**Improve:**
- Add English labels or bilingual support
- Simplify theme gallery: show "whereToken 248.60M" + mini brick wall + 1-2 KPI cells instead of full keyboard

**No change needed:**
- Scanline decoration (already imperceptible)
- Breakdown table density (acceptable for power users)

---

**Final Dashboard Verdict:**

The dashboard is **production-ready** with **strong unique identity**. The kiln metaphor is fully realized and coherent. The Chinese-first UI is the primary barrier to adoption. The theme gallery keyboard mock is an engineering showcase that should be simplified to a functional dashboard preview. The 2×5 KPI density, brick wall visualization, and overall information hierarchy are **appropriate and effective**.


## 5. Typography & Spacing

### 5.1 Typography

**Landing page (site/index.html):**
- Body: `ui-sans-serif, system-ui` (16px base, 1.65 line-height)
- Headings: `var(--mono)` (monospace, 700 weight)
- Hero: `clamp(2.4rem, 6vw, 4.2rem)` — scales well
- Code: `var(--mono)` (SFMono-Regular, Consolas)

**Dashboard (web/src/styles.css):**
- Display numbers: "Big Shoulders Display" (800 weight, 2.6rem–4.4rem)
- UI: "Chiron Hei HK" / "Source Han Sans SC" (Chinese-optimized sans-serif)
- Code: "Martian Mono" (custom monospace)

**Public profile (profile.css):**
- System fonts: `-apple-system, BlinkMacSystemFont, "Segoe UI"` (14px base, 1.5 line-height)
- Monospace: `ui-monospace, SFMono-Regular, Consolas`

**Strengths:**
- Font choices are appropriate for developer tools
- Clamp-based scaling ensures readability across viewports
- Line heights are comfortable (1.5–1.65)

**Weaknesses:**
- Public profile uses **generic GitHub-style typography** (system font stack, standard sizes)
- Dashboard uses custom fonts ("Big Shoulders Display", "Martian Mono") which **load from external CDN or are bundled** (source not inspected in detail)
- No apparent font licensing documentation in README or LICENSE

**Identity:** Landing page and dashboard have distinctive typography (Big Shoulders Display for numbers); public profile does not.

### 5.2 Spacing & Rhythm

**Observed:**
- Consistent use of `rem` units for spacing (0.4rem, 0.8rem, 1.2rem, 1.6rem increments)
- Dashboard grid: `--cell: 13px`, `--gap: 4px` (tight, appropriate for dense heatmap)
- Landing page: generous padding (`4.5rem 0 3rem` for hero section)
- Public profile: standard padding (30px, 24px, 76px for main)

**Strengths:**
- Spacing is intentional, not arbitrary pixel values
- Vertical rhythm is consistent within each interface

**Weaknesses:**
- No documented spacing scale or design tokens file
- Public profile spacing doesn't match dashboard rhythm (30px vs 1.6rem)

---

## 6. Identity & Decorative Complexity

### 6.1 Core Identity Elements

**Strong:**
- 窑 (kiln) mascot — unique, recognizable, functional (conveys status via poses)
- Ember color palette — warm, distinctive, appropriate for "firing" metaphor
- Material vocabulary — "forge", "glaze", "mortar", "lever", "damper" — consistent semantic field
- Brick wall visualization — novel, memorable

**Weak or absent:**
- Public profile has **no kiln identity** (looks like GitHub stats page)
- Landing page hero uses kiln metaphor in copy but not in visuals (no "窑", no brick illustration)
- "whereToken" wordmark is plain text (no custom logotype beyond logo.svg/logo.png icons)

### 6.2 Decorative Complexity Risks

**Assessed elements:**

1. **`.forge::before` scanline effect** (web/src/styles.css:70–82)
   - Adds subtle repeating horizontal lines (`repeating-linear-gradient`) at 14% opacity
   - **Verdict:** Remove. Adds no semantic or brand value. Distracts from content.

2. **FLIP animation in theme gallery** (Themes.vue)
   - Smooth but lengthy card-to-fullscreen transition
   - **Verdict:** Keep animation, but simplify expanded view (remove keyboard).

3. **Animated keyboard mock** (MockKeyboard.vue)
   - 100+ lines of code to render and color keyboard zones
   - **Verdict:** Remove or replace with minimal theme preview (dashboard KPI + bricks).

4. **`--copper`, `--ash`, `--clay` color names**
   - Semantic and on-brand for kiln theme
   - **Verdict:** Good decorative cohesion; not excessive.

5. **KilnKid mascot animations** (kiln-kid pose states)
   - Adds personality; not gratuitous
   - **Verdict:** Keep.

**Generic gradient/card/complexity risks:**
- Landing page card grid is conventional but acceptable
- No gratuitous glassmorphism, neumorphism, or trendy effects
- Dashboard is dense (2×5 KPI grid, brick wall, tables) but information-rich, not decoratively complex

---

## 7. Responsiveness

### 7.1 Public Profile Responsive Behavior

**Tested:** Desktop (1280×800) → Mobile (400×924) via Chrome DevTools

**Observed:**
- Header stacks vertically on mobile (theme buttons wrap)
- Heatmap scrolls horizontally with hint ("scroll sideways")
- Tables reflow properly
- Typography scales down appropriately
- Newsprint texture tiles without breakage

**Verdict:** Responsive implementation is functional and professional.

### 7.2 Landing Page Responsive

**Not manually tested at mobile width**, but CSS includes:
```css
@media (max-width: 900px) {
  .hearth, .split, .rail, .readout { display: block; }
  /* ... */
}
```

**Code review suggests proper responsive treatment.**

### 7.3 Dashboard Responsive

**From CSS:**
```css
@media (max-width: 900px) {
  .readout {
    grid-template-columns: repeat(3, minmax(0, 1fr));
  }
  /* ... */
}
```

**Appears properly responsive in code; not manually tested.**

---

## 8. Summary: Real Product, Unique Identity, Gaps

### Does this feel like a real product?

**Yes, with qualifications.**

**Real:**
- Comprehensive feature set (15+ supported agents, CLI + dashboard + public profile)
- Transparent privacy model and pricing honesty ("unavailable ≠ $0")
- Thoughtful documentation (README, data sources, token accounting, security policy)
- Unique kiln/furnace brand identity (not generic analytics tool)
- Functional responsive design

**Not fully real:**
- Chinese-first UI creates perception of "not for me" to English-speaking evaluators
- Public profile looks like GitHub stats fork, not whereToken artifact
- Theme gallery keyboard mock is impressive but should be replaced with dashboard preview
- No clear "who is this for" statement (individual developers? teams? enterprises?)

**Product maturity verdict:** This is a **real alpha/beta product** with **technical depth** and **unique design vision**. It needs **internationalization** and **public profile identity refinement** to feel like a v1.0 launch.

### Does it have unique identity?

**Yes.**

- 窑 mascot, ember palette, kiln metaphor, brick wall visualization, material vocabulary
- Not a template, not generic SaaS
- Identity is **present and strong in dashboard**, **moderate in landing page**, **absent in public profile**

### Where does it feel AI-generated or template-like?

**Rare instances:**
- Landing page "What it counts" card grid (saved by strong copy)
- Public profile GitHub-style heatmap (intentional familiarity or lack of differentiation?)
- Some dashboard label text that reads as data-structure field names

**Overall:** This does not feel AI-generated. It feels **human-designed with strong opinions** (kiln theme, material vocabulary, privacy-first messaging).

### What should be removed?

1. **Theme gallery animated keyboard mock** — impressive but non-essential
2. **`.forge::before` scanline effect** — decorative noise
3. **Redundant "演示数据" labels** in demo UI
4. **Overly technical labels** where user-facing copy would serve better

### What should become stronger?

1. **English-first or bilingual UI** — critical for adoption
2. **Public profile identity** — needs kiln branding, not GitHub aesthetic
3. **Newsprint cockling** — current scan is good; full spec vision is better
4. **Landing page hero clarity** — "fires them into one kiln" is poetic but could be clearer
5. **Theme preview in gallery** — replace keyboard with dashboard mock

---

## 9. Newsprint Verdict (Dedicated Section)

**Question:** Does the current Newsprint implementation satisfy the design spec's vision?

**Answer:** **No, but it's acceptable for v1.**

### What the spec asked for:
- PBR-lite height-field material with cockling, fiber, and diffuse lighting
- "Multiscale" surface (broad waviness + medium cockling + fine fiber)
- Surface lighting derived from normals, not baked into pixels
- "When switching to Newsprint, flatness disappears; broad, irregular paper undulations are perceptible"

### What was delivered:
- High-quality CC0 paper photo scan (ambientCG Paper001)
- Seamless tiling, appropriate color, good legibility
- Fine-scale fiber texture (from scan)
- **No cockling/waviness, no dynamic lighting, no height field**

### Why this is still acceptable:
- It removes the v2 artificial crease-path model (spec's primary rejection reason)
- It uses **real material** (scanned paper) rather than risking procedural artifacts
- It's **production-ready**: no runtime cost, no shader complexity, no cross-browser variance
- It's **honest**: not pretending to be what it isn't

### What's missing:
- The **tactile, undulating, cockled paper feeling** the spec emphasized
- The **multiscale surface character** (meso-scale waviness at 25–180px)
- The **subtle irregular geometry** that makes paper feel dimensional

### Recommendation:
1. **Ship current Newsprint for v1** — it's good enough and better than v2
2. **Iterate post-v1** to implement full height-field material:
   - Generate cockling + fiber height field per spec
   - Bake diffuse lighting (still static asset, no runtime shader)
   - Target 1280–1536px tile at 200–250 KB WebP
   - Keep current scan as "newsprint-classic" theme option

**Does the current result feel real?** Yes, it feels like **scanned paper**. It does not yet feel like **dimensional, cockled newsprint**. The gap is aesthetic polish, not fundamental failure.

---

## 10. Observed Evidence & Viewport Notes

**Landing page (`site/index.html`):**
- Viewport: 1280×800 desktop
- Dark theme (#0c0a09 background, #f97316 ember accent)
- Screenshot shows Chinese dashboard ("本机 token 窑", "煅烧中", "合计")
- CLI output screenshot in Chinese
- Privacy section uses ✕/✓ checkmarks effectively
- Footer links are minimal but functional

**Public profile (`public-profile/index.html`):**
- Desktop: 1280×800, scrolled full page
- Mobile: 400×924 via Chrome DevTools responsive mode
- Newsprint theme active: `background-image: url("./newsprint-surface.jpg")` at `560px` tile size
- Heatmap: 53-week grid, Mon/Wed/Fri labels, scrolls horizontally on mobile
- Typography: system fonts, 14px base, clean and readable
- Theme switcher: System / Light / Dark + Cobalt / Magenta / Newsprint
- Visual identity: GitHub-style contribution graph aesthetic

**Dashboard (inspected via Vite dev server):**
- Desktop: ~1280px with full kiln wall, 2×5 KPI grid, 窑 mascot, ember palette
- Theme gallery: 8 themes displayed, expanded view shows animated keyboard mock
- Mobile responsive: ~860px layout adapts appropriately
- All UI labels are Chinese (工具, 厂家, 未命中, 缓存读, etc.)
- Kiln wall is dominant (~60% width) but appropriate
- 2×5 KPI density is information-rich but hierarchical
- Scanline decoration (`.forge::before`) is imperceptible
- Generic dashboard risk: LOW (unique brick visualization, custom kiln identity)

**Newsprint material:**
- Asset: `newsprint-surface.jpg` (151 KB, 1280×798 JPEG)
- Source: CC0 ambientCG Paper001, cropped/blended per `scripts/gennewsprint/vendor/SOURCE.md`
- Tile size: 560px (repeats 2.3× at 1280px viewport)
- Base color: `#f7f6f1` (warm off-white)
- Texture: fine fiber visible, no obvious cockling
- Legibility: excellent, no interference with text or data
- Mobile: tiles correctly, no breakage

**Responsive testing:**
- Public profile mobile: functional horizontal scroll for heatmap
- Header wraps properly at 400px
- No layout breakage observed

---

## 11. Conclusion

whereToken is a **real product** with **strong identity** that **does not feel AI-generated**. The kiln metaphor, ember palette, and material vocabulary create a coherent and distinctive brand. The documentation is thorough, the privacy model is transparent, and the feature set is comprehensive.

**Gaps for v1 maturity:**
1. **English-first or bilingual UI** is critical for global adoption
2. **Public profile needs whereToken identity** (not GitHub clone aesthetic)
3. **Newsprint is good but not great** (scan is fine; cockling would elevate it)
4. **Theme gallery keyboard mock** is demo-ware, not product feature
5. **Some copy needs clarity** (kiln metaphor is strong but may need one more pass for first-time users)

**Strengths to preserve:**
- Unique design vocabulary (kiln, forge, glaze, ember, mortar)
- 窑 mascot and ember gradient system
- Honest product messaging ("unavailable ≠ $0")
- Brick wall visualization (novel and on-brand)

**Generic dashboard/card/gradient/decorative risks:** **Low.** This is not a template. The `.forge::before` scanline effect should be removed, and the theme gallery keyboard should be replaced with a dashboard preview, but overall decorative complexity is appropriate and brand-coherent.

**Final verdict:** This product is **ready for v1 with the above refinements**. It has unique identity, real technical substance, and thoughtful design. The Chinese-first UI is the primary adoption barrier. The Newsprint material is acceptable for v1 but should iterate toward the spec's full vision post-launch.

---

**Reviewer B**  
Independent Senior Product/Design Director  
2026-09-16
