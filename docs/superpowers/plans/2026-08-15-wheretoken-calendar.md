# Kiln-wall calendar Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Backend emits per-series daily buckets, intensity levels, peak, and streaks; Vue replaces the newsprint dashboard with a dark kiln contribution wall that switches on the existing tool/vendor axes.

**Architecture:** `metric.BuildCalendar(events, loc, now)` buckets merged `UsageEvent`s by local date into `calendar.all` / `by_source` / `by_vendor`. `Aggregate` calls it. `scan.EncodeSummary` JSON-encodes it. Vue reads `calendar` and never recomputes buckets, peak, or streaks.

**Tech Stack:** Go 1.25 `testing`, Vue 3 + Vite + TypeScript + Pinia, CSS Grid (no ECharts calendar). Fonts: Big Shoulders Display, Chiron Hei HK, Martian Mono.

## Global Constraints

- Token math is `int64` until display; display divides by `1_000_000` and suffixes `M` via `FormatM`.
- `total = miss + cache_read + cache_create + output`.
- `sum(BySource.Total) == All.Total == sum(ByVendor.Total)`.
- `sum(calendar.all.days.total) == all.total` (same for each source/vendor series).
- Week start is Monday. Dates are `time.Local` (tests use `Asia/Shanghai`).
- Current streak includes today only if today has usage; otherwise consecutive days ending yesterday.
- Intensity quartiles are per filtered series, never a global max.
- Read-only sources. Bind `127.0.0.1` only.
- Git commits via `scripts/commit-no-ai.sh` only. No `Co-authored-by`.
- TDD: no production code before a failing test. Watch the fail.
- Spec: `docs/superpowers/specs/2026-08-15-wheretoken-calendar-design.md`.

---

## File map (locked)

| Path | Responsibility |
|------|----------------|
| `internal/metric/calendar.go` | `Day`, `Calendar`, `Series`, `Stats`, `BuildCalendar` |
| `internal/metric/calendar_test.go` | TDD for merge/peak/streak/filter/conservation/levels |
| `internal/metric/summary.go` | `Summary.Calendar`; `Aggregate` calls `BuildCalendar` |
| `internal/scan/scan.go` | JSON encode `calendar` |
| `internal/httpapi/httpapi_test.go` | payload includes `calendar.week_start` |
| `web/src/types.ts` | `Calendar`, `Series`, `Day`, `Stats` |
| `web/src/components/KilnWall.vue` | 53×7 CSS grid |
| `web/src/components/FoundryMarks.vue` | peak + streaks |
| `web/src/components/AxisDamper.vue` | 合计 / 工具 / 厂家 |
| `web/src/grid.ts` | layout-only: window → cells (empty/future); no stats math |
| `web/src/grid.test.ts` | empty vs future |
| `web/src/App.vue` | kiln layout |
| `web/src/styles.css` | dark kiln identity |
| `web/index.html` | font links |
| `web/src/components/ShareBars.vue` | delete |
| `web/package.json` | drop `echarts` if unused |

---

### Task 1: Same local day merges; conservation

**Files:**
- Create: `internal/metric/calendar.go`
- Test: `internal/metric/calendar_test.go`

**Interfaces:**
- Consumes: `[]event.UsageEvent` (already request-merged by caller in later tasks; this task may merge by date only)
- Produces: `BuildCalendar(events []event.UsageEvent, loc *time.Location, now time.Time) Calendar` with `All.Days []Day` where `Day` has `Date string`, `Miss, CacheRead, CacheCreate, Output, Total int64`

- [ ] **Step 1: Write the failing test**

```go
package metric

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func ts(loc *time.Location, y, m, d, hh, mm int) time.Time {
	return time.Date(y, time.Month(m), d, hh, mm, 0, 0, loc)
}

func TestBuildCalendarMergesSameLocalDay(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 15, 1, 0), Miss: 10, CacheRead: 90, Output: 5},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 23, 0), Miss: 20, Output: 3},
	}
	cal := BuildCalendar(events, loc, now)
	if len(cal.All.Days) != 1 {
		t.Fatalf("days=%d", len(cal.All.Days))
	}
	d := cal.All.Days[0]
	if d.Date != "2026-08-15" {
		t.Fatalf("date=%s", d.Date)
	}
	if d.Total != 10+90+5+20+3 {
		t.Fatalf("total=%d", d.Total)
	}
	if d.Miss != 30 || d.CacheRead != 90 || d.Output != 8 {
		t.Fatalf("fields %+v", d)
	}
}

func TestBuildCalendarConservationMatchesAggregate(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 100, CacheRead: 900, Output: 10},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 50, Output: 5},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Timestamp: ts(loc, 2026, 8, 15, 11, 0), Miss: 20, CacheRead: 80, Output: 3},
	}
	sum := Aggregate(events, nil)
	cal := BuildCalendar(events, loc, now)
	var daySum int64
	for _, d := range cal.All.Days {
		daySum += d.Total
	}
	if daySum != sum.All.Total() {
		t.Fatalf("days=%d all=%d", daySum, sum.All.Total())
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metric -run 'TestBuildCalendarMergesSameLocalDay|TestBuildCalendarConservationMatchesAggregate' -v`

Expected: FAIL compile or undefined `BuildCalendar`

- [ ] **Step 3: Write minimal implementation**

Add types and `BuildCalendar` that groups by local date for `All` only. `Aggregate` unchanged this step except tests already call it.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metric -run 'TestBuildCalendarMergesSameLocalDay|TestBuildCalendarConservationMatchesAggregate' -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metric/calendar.go internal/metric/calendar_test.go
bash scripts/commit-no-ai.sh "feat: bucket usage events into local calendar days"
```

---

### Task 2: Empty days break streaks; peak picks max; current streak ends yesterday

**Files:**
- Modify: `internal/metric/calendar.go`
- Test: `internal/metric/calendar_test.go`

**Interfaces:**
- Produces: `Series.Stats` with `PeakDate string`, `PeakTotal int64`, `CurrentStreak int`, `LongestStreak int`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildCalendarEmptyDayBreaksStreak(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "1", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 1},
		{Source: "kimi", Vendor: "moonshot", RequestID: "2", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 1},
		// 13 empty
		{Source: "kimi", Vendor: "moonshot", RequestID: "4", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 1},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.LongestStreak != 2 {
		t.Fatalf("longest=%d", cal.All.Stats.LongestStreak)
	}
	if cal.All.Stats.CurrentStreak != 1 {
		t.Fatalf("current=%d want 1 (yesterday only; today empty)", cal.All.Stats.CurrentStreak)
	}
}

func TestBuildCalendarPeakPicksMaxTotalDay(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 10, 10, 0), Miss: 10},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 50},
		{Source: "kimi", Vendor: "moonshot", RequestID: "c", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 20},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.PeakDate != "2026-08-11" || cal.All.Stats.PeakTotal != 50 {
		t.Fatalf("peak=%s %d", cal.All.Stats.PeakDate, cal.All.Stats.PeakTotal)
	}
}

func TestBuildCalendarCurrentStreakIncludesTodayWhenUsed(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", RequestID: "a", Timestamp: ts(loc, 2026, 8, 14, 10, 0), Miss: 1},
		{Source: "kimi", Vendor: "moonshot", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 1},
	}
	cal := BuildCalendar(events, loc, now)
	if cal.All.Stats.CurrentStreak != 2 {
		t.Fatalf("current=%d", cal.All.Stats.CurrentStreak)
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metric -run 'TestBuildCalendarEmptyDayBreaksStreak|TestBuildCalendarPeakPicksMaxTotalDay|TestBuildCalendarCurrentStreakIncludesTodayWhenUsed' -v`

Expected: FAIL on zero stats

- [ ] **Step 3: Write minimal implementation**

Compute peak and streaks over local dates from first event through `now`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metric -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metric/calendar.go internal/metric/calendar_test.go
bash scripts/commit-no-ai.sh "feat: compute peak and usage streaks per calendar series"
```

---

### Task 3: Vendor MiniMax filter; per-series quartiles

**Files:**
- Modify: `internal/metric/calendar.go`
- Test: `internal/metric/calendar_test.go`

**Interfaces:**
- Produces: `Calendar.BySource map[string]Series`, `Calendar.ByVendor map[string]Series`; `Day.Level int`

- [ ] **Step 1: Write the failing test**

```go
func TestBuildCalendarVendorMinimaxUsesOnlyThoseEvents(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "a", Timestamp: ts(loc, 2026, 8, 15, 10, 0), Miss: 100},
		{Source: "claude", Vendor: "minimax", RequestID: "b", Timestamp: ts(loc, 2026, 8, 15, 11, 0), Miss: 40},
	}
	cal := BuildCalendar(events, loc, now)
	mm := cal.ByVendor["minimax"]
	if len(mm.Days) != 1 || mm.Days[0].Total != 40 {
		t.Fatalf("minimax days=%v", mm.Days)
	}
	if mm.Stats.PeakTotal != 40 {
		t.Fatalf("minimax peak=%d", mm.Stats.PeakTotal)
	}
	var all int64
	for _, d := range cal.All.Days {
		all += d.Total
	}
	if all != 140 {
		t.Fatalf("all=%d", all)
	}
}

func TestBuildCalendarLevelsUseSeriesQuartilesNotGlobalMax(t *testing.T) {
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12, 0)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "big", Timestamp: ts(loc, 2026, 8, 10, 10, 0), Miss: 1_000_000},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s1", Timestamp: ts(loc, 2026, 8, 11, 10, 0), Miss: 10},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s2", Timestamp: ts(loc, 2026, 8, 12, 10, 0), Miss: 10},
		{Source: "opencode", Vendor: "anthropic", RequestID: "s3", Timestamp: ts(loc, 2026, 8, 13, 10, 0), Miss: 10},
	}
	cal := BuildCalendar(events, loc, now)
	oc := cal.BySource["opencode"]
	if len(oc.Days) != 3 {
		t.Fatalf("opencode days=%d", len(oc.Days))
	}
	for _, d := range oc.Days {
		if d.Level != 2 {
			t.Fatalf("equal small days should be mid level, got %d on %s", d.Level, d.Date)
		}
	}
	if cal.All.Days == nil {
		t.Fatal("all days")
	}
}
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/metric -run 'TestBuildCalendarVendorMinimax|TestBuildCalendarLevels' -v`

Expected: FAIL missing maps/levels

- [ ] **Step 3: Write minimal implementation**

Fill `BySource` / `ByVendor`. Quartile levels per series as spec §5.2.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./internal/metric -v`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metric/calendar.go internal/metric/calendar_test.go
bash scripts/commit-no-ai.sh "feat: split calendar series by tool and vendor"
```

---

### Task 4: Wire calendar into Aggregate JSON

**Files:**
- Modify: `internal/metric/summary.go`
- Modify: `internal/scan/scan.go`
- Test: `internal/httpapi/httpapi_test.go`

**Interfaces:**
- Produces: `Summary.Calendar`; JSON key `calendar` with `week_start: "monday"`

- [ ] **Step 1: Write the failing test**

Extend `TestSummaryMatchesScan` (or add `TestSummaryIncludesCalendar`) to unmarshal `calendar.week_start` and `calendar.all.stats`.

```go
var payload struct {
	All struct {
		Total int64 `json:"total"`
	} `json:"all"`
	Calendar struct {
		WeekStart string `json:"week_start"`
		All       struct {
			Stats struct {
				PeakTotalM string `json:"peak_total_m"`
			} `json:"stats"`
		} `json:"all"`
	} `json:"calendar"`
}
// week_start must be "monday"
```

- [ ] **Step 2: Run test to verify it fails**

Run: `go test ./internal/httpapi -run TestSummaryIncludesCalendar -v`

Expected: FAIL empty week_start

- [ ] **Step 3: Write minimal implementation**

`Aggregate` sets `sum.Calendar = BuildCalendar(merged, time.Local, time.Now())`. Encode `calendar` with `FormatM` on day and peak fields. `window_from`/`window_to` Monday-aligned 53 weeks ending `now`.

- [ ] **Step 4: Run test to verify it passes**

Run: `go test ./...`

Expected: PASS

- [ ] **Step 5: Commit**

```bash
git add internal/metric/summary.go internal/scan/scan.go internal/httpapi/httpapi_test.go
bash scripts/commit-no-ai.sh "feat: expose calendar series on scan JSON and /api/summary"
```

---

### Task 5: Vue kiln wall (hero) + identity

**Files:**
- Create: `web/src/grid.ts`, `web/src/grid.test.ts`, `web/src/components/KilnWall.vue`, `web/src/components/FoundryMarks.vue`, `web/src/components/AxisDamper.vue`
- Modify: `web/src/types.ts`, `web/src/App.vue`, `web/src/styles.css`, `web/index.html`, `web/src/stores/summary.ts`
- Delete: `web/src/components/ShareBars.vue`
- Modify: `web/package.json` (remove echarts)

**Interfaces:**
- Consumes: `payload.calendar`
- Produces: layout cells from `window_from`/`window_to` + sparse days; no peak/streak math

- [ ] **Step 1: Write the failing test**

```ts
import { describe, expect, it } from 'vitest'
import { layoutCells } from './grid'
import type { Day } from './types'

describe('layoutCells', () => {
  it('marks missing in-window days empty and dates after today future', () => {
    const days: Day[] = [
      {
        date: '2026-08-14',
        miss: 1, cache_read: 0, cache_create: 0, output: 0, total: 1,
        miss_m: '0.0000 M', cache_read_m: '0.00 M', cache_create_m: '0.00 M', output_m: '0.00 M', total_m: '0.0000 M',
        level: 2,
      },
    ]
    const cells = layoutCells({
      windowFrom: '2026-08-10',
      windowTo: '2026-08-16',
      today: '2026-08-15',
      weekStart: 'monday',
      days,
    })
    const byDate = Object.fromEntries(cells.map((c) => [c.date, c.kind]))
    expect(byDate['2026-08-14']).toBe('lit')
    expect(byDate['2026-08-13']).toBe('empty')
    expect(byDate['2026-08-16']).toBe('future')
  })
})
```

- [ ] **Step 2: Run test to verify it fails**

Run: `cd web && npm test`

Expected: FAIL cannot find module `./grid`

- [ ] **Step 3: Write grid + Vue kiln identity**

Implement `layoutCells`. Rebuild `App.vue`: wall hero, foundry marks, damper, restyled tables. Dark CSS variables, new fonts, delete ShareBars, `npm uninstall echarts`.

- [ ] **Step 4: Run test to verify it passes**

Run: `cd web && npm test && npx vue-tsc --noEmit && npm run build`

Expected: PASS / build ok

- [ ] **Step 5: Commit**

```bash
git add web
bash scripts/commit-no-ai.sh "feat: render kiln-wall calendar as the dashboard hero"
```

---

### Task 6: Verify conservation on fixtures and live disk

**Files:**
- Modify: `scripts/verify-local.sh` if JSON shape needs a calendar conservation check

- [ ] **Step 1: Add calendar conservation to verify-local.sh**

```python
days = scan.get("calendar", {}).get("all", {}).get("days") or []
day_sum = sum(int(d["total"]) for d in days)
if day_sum != all_t:
    raise SystemExit(f"calendar conservation fail days={day_sum} all={all_t}")
```

- [ ] **Step 2: Run**

Run: `go test ./... && cd web && npm test && cd .. && bash scripts/verify-local.sh`

Expected: all PASS, calendar conservation ok

- [ ] **Step 3: Commit if script changed**

```bash
git add scripts/verify-local.sh
bash scripts/commit-no-ai.sh "test: assert calendar day totals match grand total"
```

---

## Self-review

1. Spec coverage: merge, empty streak break, peak, MiniMax filter, conservation, Monday, local TZ, current-streak-yesterday, per-series quartiles, kiln UI, no ECharts calendar — each has a task.
2. No TBD placeholders.
3. Types: `BuildCalendar(events, loc, now) Calendar`; JSON `calendar.week_start`, `calendar.all.stats`, `by_source`/`by_vendor` maps match slice ids.
