package card

import (
	"fmt"
	"math/rand"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

// DemoLoc is the fixed timezone for the committed Vibe Coding Wall fixture.
func DemoLoc() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

// DemoNow is Wednesday 2026-09-09 15:00 in DemoLoc, so the wall has
// four future cells (Thu–Sun) and a stable 53-week window.
func DemoNow() time.Time {
	return time.Date(2026, 9, 9, 15, 0, 0, 0, DemoLoc())
}

// DemoEvents returns the synthetic ledger used by tests and
// scripts/gencarddemo. It is invented; it never reads HOME or real files.
func DemoEvents() []event.UsageEvent {
	loc := DemoLoc()
	now := DemoNow()
	today := time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, loc)
	rng := rand.New(rand.NewSource(7))
	var evs []event.UsageEvent
	add := func(src, vend, model string, day time.Time, miss, cr, out int64) {
		hour := 9 + rng.Intn(9)
		evs = append(evs, event.UsageEvent{
			Source:     src,
			Vendor:     vend,
			Model:      model,
			RequestID:  fmt.Sprintf("%s-%s-%d", src, day.Format("20060102"), len(evs)),
			Timestamp:  day.Add(time.Duration(hour) * time.Hour),
			Miss:       miss,
			CacheRead:  cr,
			Output:     out,
			Quality:    event.QualityAuthoritative,
			Derivation: event.DeriveRaw,
		})
	}
	vac0 := time.Date(2026, 5, 1, 0, 0, 0, 0, loc)
	vac1 := time.Date(2026, 5, 10, 0, 0, 0, 0, loc)
	for d := 420; d >= 0; d-- {
		day := today.AddDate(0, 0, -d)
		if !day.Before(vac0) && !day.After(vac1) {
			continue
		}
		wd := day.Weekday()
		weekend := wd == time.Saturday || wd == time.Sunday
		if weekend && rng.Float64() > 0.32 {
			continue
		}
		if !weekend || rng.Float64() < 0.22 {
			add("claude", "anthropic", "claude-opus-4.6", day,
				int64(90_000+rng.Intn(280_000)),
				int64(rng.Intn(420_000)),
				int64(10_000+rng.Intn(45_000)))
		}
		if rng.Float64() < 0.58 {
			add("kimi", "moonshot", "kimi-k2.5", day,
				int64(40_000+rng.Intn(140_000)),
				int64(rng.Intn(90_000)),
				int64(6_000+rng.Intn(22_000)))
		}
		if wd == time.Wednesday || rng.Float64() < 0.28 {
			add("codex", "openai", "gpt-5.3-codex", day,
				int64(28_000+rng.Intn(95_000)),
				0,
				int64(8_000+rng.Intn(26_000)))
		}
		if rng.Float64() < 0.16 {
			add("grok", "xai", "grok-4.6-build", day,
				int64(12_000+rng.Intn(55_000)),
				0,
				int64(3_000+rng.Intn(14_000)))
		}
		if rng.Float64() < 0.11 {
			add("cursor", "cursor", "claude-sonnet-4.6", day,
				int64(8_000+rng.Intn(36_000)),
				int64(rng.Intn(24_000)),
				int64(2_000+rng.Intn(9_000)))
		}
	}
	peak := time.Date(2026, 3, 12, 0, 0, 0, 0, loc)
	add("claude", "anthropic", "claude-opus-4.6", peak, 2_400_000, 6_200_000, 210_000)
	return evs
}

// DemoCard is NewView over DemoEvents at DemoNow. Version is pinned so the
// committed golden SVG stays byte-identical across rebuilds.
func DemoCard() PublicCard {
	sum := metric.AggregateAt(DemoEvents(), nil, DemoNow(), DemoLoc())
	return NewView(sum, StatusOf(sum), "dev")
}
