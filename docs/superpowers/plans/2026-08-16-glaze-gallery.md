# 釉厅 4×2 + FLIP 展开 Implementation Plan

> **For agentic workers:** Execute inline in this session (parent already dispatched the work onto `main` and asked to push `origin/main`). TDD. Commit with `scripts/commit-no-ai.sh` only.

**Goal:** Eight considered glazes in a 4×2 hall; click Flip-expands a homepage mock; 应用 persists and returns home.

**Architecture:** Theme packs gain `blurb` + `chrome`. Gallery is one `Themes.vue` bound to `/themes/:id?`. GSAP Flip moves the same card; mock is a static home sketch using real wall/lever classes.

**Tech Stack:** Vue 3, Vue Router, GSAP 3 Flip, Vitest, existing theme CSS variables.

## Global Constraints

- Commit via `scripts/commit-no-ai.sh`; no `Co-authored-by`; do not change git config.
- `opt.md` stays gitignored; new glaze rationale goes there only.
- Home kiln wall stays flat 2D (no box-shadow / 3D / glow). Radius/type/brick shape may vary.
- `prefers-reduced-motion: reduce` skips Flip/fade.
- Do not persist theme on mere expand.
- Contrast: bone/ash/ember-4/warn on void ≥ 4.5.

---

### Task 1: Theme catalog (drop 青/霜, add 漫/端, chrome tokens)

**Files:**
- Modify: `web/src/themes/manifest.ts`
- Modify: `web/src/themes/themes.test.ts`
- Modify: `web/index.html` (boot `allowed` + font links)
- Test: `web/src/themes/themes.test.ts`

- [ ] **Step 1: Write the failing catalog tests** (ids, fallback, contrast, chrome, blurbs)
- [ ] **Step 2: Run `cd web && npx vitest run src/themes/themes.test.ts` — expect RED**
- [ ] **Step 3: Minimal manifest + boot list + fonts to pass**
- [ ] **Step 4: Re-run until GREEN**

### Task 2: Wall/home consume chrome variables

**Files:**
- Modify: `web/src/styles.css` (`.brick` / `.lever` / `.wall` / `.legend i` use vars)
- Modify: `web/src/themes/themes.test.ts` (flat-wall assertions)

- [ ] Failing test: bricks use `var(--brick-radius)`, not hardcoded `2px`
- [ ] Implement CSS variable wiring
- [ ] GREEN: flat 2D bans still hold (no box-shadow, no translateY)

### Task 3: Gallery motion helper

**Files:**
- Create: `web/src/themes/galleryMotion.ts`
- Create: `web/src/themes/galleryMotion.test.ts`
- Modify: `web/package.json` (add `gsap`)

- [ ] Failing tests: reduced-motion path does not throw; expand does not persist
- [ ] Install gsap; implement Flip timeline + reduced branch
- [ ] GREEN

### Task 4: Themes page 4×2 + expand mock + routes

**Files:**
- Modify: `web/src/pages/Themes.vue`
- Modify: `web/src/router.ts`
- Modify: `web/src/styles.css` (shelf 4-col, expanded layout)
- Create: `web/src/themes/mockWall.ts` (+ test if pattern is non-trivial)
- Modify: `web/src/themes/themes.test.ts` (structure, mock, 应用)

- [ ] Failing tests for 4-col / 8 cards / mock / 应用 persist+leave
- [ ] Implement page + mock + Flip loop + `/themes/:id?`
- [ ] GREEN

### Task 5: Copy, README, calendar spec, opt.md, verify, commit, push

**Files:**
- Modify: `README.md`, calendar spec glaze paragraph
- Modify: `opt.md` (local only)
- Verify: `cd web && npm test && npm run build`

- [ ] Docs match 8 glazes
- [ ] Verify evidence before claiming
- [ ] `scripts/commit-no-ai.sh` then `git push origin main`
