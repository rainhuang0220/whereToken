package profile

import (
	"time"

	"github.com/rainhuang0220/whereToken/internal/metric"
)

// Bucket thresholds. Every dimension of the profile vector lands in bucket
// 0 (low), 1 (mid), or 2 (high); only extreme buckets drive traits, so a
// small metric change inside a bucket never moves the portrait. Thresholds
// align with internal/insight where it measures the same thing.
const (
	// minProfileTokens: below this a window is noise → StateInsufficient.
	// Same floor as insight.evalMinTokens.
	minProfileTokens int64 = 100_000

	// intensity, tokens per active day:
	// < 0.5M → light, [0.5M, 5M] → mid, > 5M → high.
	intensityLightPerDay = 500_000.0
	intensityHighPerDay  = 5_000_000.0 // insight.highUsagePerDayTokens

	// cost, API-equivalent USD per active day (only when priced):
	// < $5 → efficient, [$5, $25) → mid, ≥ $25 → high.
	costEfficientPerDayUSD = 5.0
	costHighPerDayUSD      = 25.0 // insight.highCostPerDayUSD

	// model diversity, distinct labeled models in DrillAll.Models:
	// ≤ 1 → single, 2–3 → some, ≥ 4 → diverse.
	modelDiverseMin = 4

	// vendor diversity, distinct known vendors in ByVendor:
	// ≤ 1 → single, 2 → some, ≥ 3 → diverse (insight.multiModelMinVendors).
	vendorDiverseMin = 3

	// cache reuse, cache_read / (miss + cache_read + cache_create) percent:
	// < 40 → low, [40, 75) → some, ≥ 75 → high (insight.cacheReuseMinHitPct).
	cacheLowHitPct  = 40.0
	cacheHighHitPct = 75.0

	// consistency, active days / window span in days:
	// < 0.3 → sporadic, [0.3, 0.7) → mixed, ≥ 0.7 → steady.
	consistencyMidPct    = 0.3
	consistencySteadyPct = 0.7
	// steadyMinActiveDays: a 1–3 day window is always "consistent" by the
	// ratio alone; steady requires real repetition (insight.steadyMinActiveDays).
	steadyMinActiveDays = 4

	// burstiness, peak calendar day / mean active day:
	// < 2 → even, [2, 4) → uneven, ≥ 4 → bursty.
	burstMidRatio  = 2.0
	burstHighRatio = 4.0

	// concentration, top labeled model's share of window tokens percent:
	// < 50 → spread, [50, 70) → leaning, ≥ 70 → concentrated
	// (insight.singleModelMinShare).
	concentrationMidPct  = 50.0
	concentrationHighPct = 70.0
)

// vector is the bucketed profile of one Summary window. Availability flags
// separate "measured as low" from "not measurable" (unpriced cost, no cache
// traffic, no labeled model) — unavailable is never treated as zero.
type vector struct {
	intensity     int
	cost          int
	costAvail     bool
	modelDiv      int
	vendorDiv     int
	cache         int
	cacheAvail    bool
	consistency   int
	burstiness    int
	concentration int
	concAvail     bool
	activeDays    int
}

func buildVector(sum metric.Summary) vector {
	tot := sum.All.Total()
	days := activeDays(sum)
	perDay := float64(tot) / float64(days)

	v := vector{activeDays: len(sum.Calendar.All.Days)}
	v.intensity = bucket(perDay, intensityLightPerDay, intensityHighPerDay, true)

	// Cost is available only when the window has a priced (complete or
	// non-zero partial) API-equivalent estimate.
	if metric.FormatCostUSD(sum.All.CostStatus, sum.All.CostMicro) != "" {
		v.costAvail = true
		v.cost = bucket(float64(sum.All.CostMicro)/1e6/float64(days), costEfficientPerDayUSD, costHighPerDayUSD, false)
	}

	models := labeledModels(sum)
	v.modelDiv = countBucket(len(models), modelDiverseMin)
	v.concAvail = len(models) > 0 && tot > 0
	if v.concAvail {
		v.concentration = bucket(100*float64(topModelTokens(models))/float64(tot), concentrationMidPct, concentrationHighPct, false)
	}

	v.vendorDiv = countBucket(knownVendorCount(sum), vendorDiverseMin)

	if hit, ok := metric.HitRate(sum.All.Miss, sum.All.CacheRead, sum.All.CacheCreate); ok {
		v.cacheAvail = true
		v.cache = bucket(hit, cacheLowHitPct, cacheHighHitPct, false)
	}

	v.consistency = bucket(consistencyRatio(sum), consistencyMidPct, consistencySteadyPct, false)
	if mean := perDay; mean > 0 {
		v.burstiness = bucket(float64(sum.Calendar.All.Stats.PeakTotal)/mean, burstMidRatio, burstHighRatio, false)
	}
	return v
}

// bucket maps a measurement to 0/1/2. highExclusive controls the upper
// boundary: intensity reads "> 5M" as high (exactly 5M stays mid), while the
// percentage/ratio dimensions read "≥" at both ends.
func bucket(x, low, high float64, highExclusive bool) int {
	if x < low {
		return 0
	}
	if highExclusive {
		if x <= high {
			return 1
		}
		return 2
	}
	if x < high {
		return 1
	}
	return 2
}

// countBucket buckets a diversity count: 0–1 → 0, one below the diverse
// floor → 1, at or above it → 2.
func countBucket(n, diverseMin int) int {
	if n <= 1 {
		return 0
	}
	if n >= diverseMin {
		return 2
	}
	return 1
}

// activeDays is the number of calendar days with usage, floored at 1 so
// per-day math never divides by zero (events without timestamps land here).
func activeDays(sum metric.Summary) int {
	if n := len(sum.Calendar.All.Days); n > 0 {
		return n
	}
	return 1
}

// consistencyRatio is active days over the window span in days (first to
// last active day inclusive). The span never underflows the active count.
func consistencyRatio(sum metric.Summary) float64 {
	days := sum.Calendar.All.Days
	if len(days) == 0 {
		return 1
	}
	span := len(days)
	if len(days) > 1 {
		first, err1 := time.Parse("2006-01-02", days[0].Date)
		last, err2 := time.Parse("2006-01-02", days[len(days)-1].Date)
		if err1 == nil && err2 == nil {
			if s := int(last.Sub(first).Hours()/24) + 1; s > span {
				span = s
			}
		}
	}
	return float64(len(days)) / float64(span)
}

// labeledModels mirrors insight.labeledModels: drill rows with a real model
// id and usage. The unlabeled fallback buckets are not a model.
func labeledModels(sum metric.Summary) []metric.Slice {
	var out []metric.Slice
	for _, s := range sum.DrillAll.Models {
		if metric.UnlabeledDrillID(s.ID) || s.Total() <= 0 {
			continue
		}
		out = append(out, s)
	}
	return out
}

// topModelTokens is the largest labeled model's window tokens. Computed as
// an explicit max instead of trusting drill sort order.
func topModelTokens(models []metric.Slice) int64 {
	var top int64
	for _, m := range models {
		if t := m.Total(); t > top {
			top = t
		}
	}
	return top
}

func knownVendorCount(sum metric.Summary) int {
	n := 0
	for _, s := range sum.ByVendor {
		if s.ID == "unknown" || s.Total() <= 0 {
			continue
		}
		n++
	}
	return n
}
