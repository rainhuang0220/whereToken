package profile

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

type tDay struct {
	date  string
	total int64
}

type tModel struct {
	id     string
	tokens int64
}

// makeSummary builds just enough of a metric.Summary for the portrait vector:
// All categories (miss/cacheRead split), calendar days (+ peak), drill models,
// and vendor rows. Fields are read independently by the vector, so the pieces
// only need to be self-consistent for the trait under test.
func makeSummary(days []tDay, models []tModel, vendors []string, cacheRead, costMicro int64, costStatus string) metric.Summary {
	var tot int64
	sum := metric.Summary{}
	for _, d := range days {
		tot += d.total
		sum.Calendar.All.Days = append(sum.Calendar.All.Days, metric.Day{Date: d.date, Miss: d.total, Total: d.total})
		if d.total > sum.Calendar.All.Stats.PeakTotal {
			sum.Calendar.All.Stats.PeakTotal = d.total
		}
	}
	sum.All = metric.Slice{
		ID:         "all",
		Label:      "合计",
		Miss:       tot - cacheRead,
		CacheRead:  cacheRead,
		CostMicro:  costMicro,
		CostStatus: costStatus,
	}
	for _, v := range vendors {
		sum.ByVendor = append(sum.ByVendor, metric.Slice{ID: v, Label: v, Miss: 1})
	}
	for _, m := range models {
		sum.DrillAll.Models = append(sum.DrillAll.Models, metric.Slice{ID: m.id, Label: m.id, Miss: m.tokens})
	}
	return sum
}

func everyDay(from, to string, perDay int64) []tDay {
	days := []tDay{}
	t, err := time.Parse("2006-01-02", from)
	if err != nil {
		panic(err)
	}
	end, err := time.Parse("2006-01-02", to)
	if err != nil {
		panic(err)
	}
	for !t.After(end) {
		days = append(days, tDay{date: t.Format("2006-01-02"), total: perDay})
		t = t.AddDate(0, 0, 1)
	}
	return days
}

func inPool(key, phrase string) bool {
	for _, p := range pools[key] {
		if p == phrase {
			return true
		}
	}
	return false
}

func phraseIndex() map[string]string {
	out := map[string]string{}
	for k, pool := range pools {
		for _, p := range pool {
			out[p] = k
		}
	}
	return out
}

func assertWellFormed(t *testing.T, name string, p Portrait) {
	t.Helper()
	if p.State != StateOK {
		t.Fatalf("%s: state=%q", name, p.State)
	}
	if p.Primary == "" || p.Primary == "—" {
		t.Fatalf("%s: primary=%q", name, p.Primary)
	}
	if len(p.Tags) > 2 {
		t.Fatalf("%s: tags=%v (max 2)", name, p.Tags)
	}
	known := phraseIndex()
	if _, ok := known[p.Primary]; !ok {
		t.Fatalf("%s: primary %q from no pool", name, p.Primary)
	}
	for _, tag := range p.Tags {
		if _, ok := known[tag]; !ok {
			t.Fatalf("%s: tag %q from no pool", name, tag)
		}
	}
	if !strings.Contains(p.Detail, "本周期") || !strings.Contains(p.Detail, "活跃") {
		t.Fatalf("%s: detail must carry window tokens and active days: %q", name, p.Detail)
	}
}

// (a) Each engineered summary renders a Primary from the expected trait pool.
func TestEvaluatePrimaryComesFromExpectedPool(t *testing.T) {
	unavail := price.StatusUnavailable
	complete := price.StatusComplete
	cases := []struct {
		name string
		sum  metric.Summary
		pool string
	}{
		{
			name: "high_cost",
			// $250 over 5 active days = $50/day → cost bucket 2.
			sum: makeSummary(everyDay("2026-08-01", "2026-08-05", 1_000_000),
				[]tModel{{"m1", 3_000_000}, {"m2", 2_000_000}},
				[]string{"anthropic"}, 2_500_000, 250_000_000, complete),
			pool: "cost_high",
		},
		{
			name: "high_intensity",
			// 6M tokens per active day → intensity bucket 2.
			sum: makeSummary(everyDay("2026-08-01", "2026-08-03", 6_000_000),
				[]tModel{{"m1", 18_000_000}},
				[]string{"anthropic"}, 9_000_000, 0, unavail),
			pool: "intensity_high",
		},
		{
			name: "model_diversity",
			// 5 labeled models, top share 30%, one vendor.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
			},
				[]tModel{{"a", 2_400_000}, {"b", 2_000_000}, {"c", 1_600_000}, {"d", 1_200_000}, {"e", 800_000}},
				[]string{"anthropic"}, 4_000_000, 0, unavail),
			pool: "model_diversity",
		},
		{
			name: "vendor_diversity",
			// 3 known vendors, only 2 models at 55/45.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
			},
				[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
				[]string{"anthropic", "openai", "google"}, 4_000_000, 0, unavail),
			pool: "vendor_diversity",
		},
		{
			name: "concentration",
			// Top labeled model carries 80% of the window.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
			},
				[]tModel{{"a", 6_400_000}, {"b", 1_600_000}},
				[]string{"anthropic"}, 4_000_000, 0, unavail),
			pool: "concentration",
		},
		{
			name: "cache_high",
			// 90% cache-read hit rate at volume.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
			},
				[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
				[]string{"anthropic"}, 7_200_000, 0, unavail),
			pool: "cache_high",
		},
		{
			name: "steady",
			// 4 of 5 window days active, everything else mid.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
			},
				[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
				[]string{"anthropic"}, 4_000_000, 0, unavail),
			pool: "steady",
		},
		{
			name: "bursty",
			// Peak day 5M vs mean 1.25M over 6 active days → ratio 4.
			sum: makeSummary([]tDay{
				{"2026-08-01", 500_000}, {"2026-08-02", 500_000},
				{"2026-08-03", 500_000}, {"2026-08-04", 500_000},
				{"2026-08-05", 500_000}, {"2026-08-10", 5_000_000},
			},
				[]tModel{{"a", 4_125_000}, {"b", 3_375_000}},
				[]string{"anthropic"}, 3_750_000, 0, unavail),
			pool: "bursty",
		},
		{
			name: "cost_efficient",
			// $4 over 4 active days = $1/day, everything else mid/low.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-08", 2_000_000},
			},
				[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
				[]string{"anthropic"}, 4_000_000, 4_000_000, complete),
			pool: "cost_efficient",
		},
		{
			name: "light",
			// 200k tokens per active day; model unlabeled, vendor unknown.
			sum: makeSummary(everyDay("2026-08-01", "2026-08-03", 200_000),
				[]tModel{{"(未标模型)", 600_000}},
				[]string{"unknown"}, 0, 0, unavail),
			pool: "intensity_light",
		},
		{
			name: "fallback_mid_defaults_light",
			// Nothing extreme and nothing cheap/light: mirrors insight's
			// "everything else that has data" → light.
			sum: makeSummary([]tDay{
				{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
				{"2026-08-03", 2_000_000}, {"2026-08-08", 2_000_000},
			},
				[]tModel{{"(未标模型)", 8_000_000}},
				[]string{"unknown"}, 4_000_000, 0, unavail),
			pool: "intensity_light",
		},
	}
	for _, c := range cases {
		p := Evaluate(c.sum, "test-seed-a")
		assertWellFormed(t, c.name, p)
		if !inPool(c.pool, p.Primary) {
			t.Fatalf("%s: primary %q not in pool %s", c.name, p.Primary, c.pool)
		}
	}
}

// A trait direction can never render from the opposite-direction pool:
// jitter stays inside the selected trait's pool for any seed.
func TestTraitDirectionNeverCrossesForAnySeed(t *testing.T) {
	high := makeSummary(everyDay("2026-08-01", "2026-08-03", 6_000_000),
		[]tModel{{"m1", 18_000_000}},
		[]string{"anthropic"}, 9_000_000, 0, price.StatusUnavailable)
	diverse := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
	},
		[]tModel{{"a", 2_400_000}, {"b", 2_000_000}, {"c", 1_600_000}, {"d", 1_200_000}, {"e", 800_000}},
		[]string{"anthropic"}, 4_000_000, 0, price.StatusUnavailable)
	for i := 0; i < 50; i++ {
		seed := fmt.Sprintf("seed-%d", i)
		if p := Evaluate(high, seed); !inPool("intensity_high", p.Primary) {
			t.Fatalf("high-intensity user rendered as %q (seed %s)", p.Primary, seed)
		}
		if p := Evaluate(diverse, seed); !inPool("model_diversity", p.Primary) {
			t.Fatalf("multi-model user rendered as %q (seed %s)", p.Primary, seed)
		}
	}
}

// (b) Same metrics + same seed → byte-identical Portrait.
func TestEvaluateDeterministicForSameSeed(t *testing.T) {
	sum := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
	},
		[]tModel{{"a", 2_400_000}, {"b", 2_000_000}, {"c", 1_600_000}, {"d", 1_200_000}, {"e", 800_000}},
		[]string{"anthropic"}, 4_000_000, 0, price.StatusUnavailable)
	p1 := Evaluate(sum, "determinism-check")
	p2 := Evaluate(sum, "determinism-check")
	if !reflect.DeepEqual(p1, p2) {
		t.Fatalf("same seed twice: %+v vs %+v", p1, p2)
	}
	raw1, err := json.Marshal(p1)
	if err != nil {
		t.Fatal(err)
	}
	raw2, err := json.Marshal(p2)
	if err != nil {
		t.Fatal(err)
	}
	if string(raw1) != string(raw2) {
		t.Fatalf("byte identity: %s vs %s", raw1, raw2)
	}
}

// (c) Same metrics, two fixed seeds → different phrasing. The two seeds are
// pinned here because they were verified to pick different phrases.
func TestEvaluateDifferentSeedsVaryPhrasing(t *testing.T) {
	sum := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
	},
		[]tModel{{"a", 2_400_000}, {"b", 2_000_000}, {"c", 1_600_000}, {"d", 1_200_000}, {"e", 800_000}},
		[]string{"anthropic"}, 4_000_000, 0, price.StatusUnavailable)
	p1 := Evaluate(sum, "portrait-seed-alpha")
	p2 := Evaluate(sum, "portrait-seed-beta")
	if p1.Primary == p2.Primary && reflect.DeepEqual(p1.Tags, p2.Tags) {
		t.Fatalf("seeds must vary phrasing: %q %v vs %q %v", p1.Primary, p1.Tags, p2.Primary, p2.Tags)
	}
	// Both stay inside the same-direction pools regardless of seed.
	if !inPool("model_diversity", p1.Primary) || !inPool("model_diversity", p2.Primary) {
		t.Fatalf("seed jitter crossed pools: %q / %q", p1.Primary, p2.Primary)
	}
}

// (d) A small metric change inside the same buckets keeps the phrasing
// identical. Detail intentionally tracks exact window stats, so it is
// excluded from phrasing identity.
func TestEvaluateSameBucketsSamePhrasing(t *testing.T) {
	base := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-05", 2_000_000},
	},
		[]tModel{{"a", 2_400_000}, {"b", 2_000_000}, {"c", 1_600_000}, {"d", 1_200_000}, {"e", 800_000}},
		[]string{"anthropic"}, 4_000_000, 0, price.StatusUnavailable)
	// +1% tokens: every bucket stays where it was.
	plus := makeSummary([]tDay{
		{"2026-08-01", 2_020_000}, {"2026-08-02", 2_020_000},
		{"2026-08-03", 2_020_000}, {"2026-08-05", 2_020_000},
	},
		[]tModel{{"a", 2_424_000}, {"b", 2_020_000}, {"c", 1_616_000}, {"d", 1_212_000}, {"e", 808_000}},
		[]string{"anthropic"}, 4_040_000, 0, price.StatusUnavailable)
	p1 := Evaluate(base, "bucket-stability")
	p2 := Evaluate(plus, "bucket-stability")
	if p1.State != p2.State || p1.Primary != p2.Primary || !reflect.DeepEqual(p1.Tags, p2.Tags) {
		t.Fatalf("phrasing moved inside the same buckets: %q %v vs %q %v",
			p1.Primary, p1.Tags, p2.Primary, p2.Tags)
	}
}

// (e) Crossing a bucket boundary changes the Primary direction.
func TestEvaluateBucketCrossingChangesDirection(t *testing.T) {
	mid := makeSummary([]tDay{{"2026-08-01", 4_000_000}},
		[]tModel{{"(未标模型)", 4_000_000}},
		[]string{"unknown"}, 2_000_000, 0, price.StatusUnavailable)
	heavy := makeSummary([]tDay{{"2026-08-01", 8_000_000}},
		[]tModel{{"(未标模型)", 8_000_000}},
		[]string{"unknown"}, 4_000_000, 0, price.StatusUnavailable)
	pMid := Evaluate(mid, "crossing")
	pHeavy := Evaluate(heavy, "crossing")
	if !inPool("intensity_light", pMid.Primary) {
		t.Fatalf("4M/day single day should fall back to light, got %q", pMid.Primary)
	}
	if !inPool("intensity_high", pHeavy.Primary) {
		t.Fatalf("8M/day should render high intensity, got %q", pHeavy.Primary)
	}

	// Cache hit rate crossing 75%: fallback light → cache_high.
	warm := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-08", 2_000_000},
	},
		[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
		[]string{"anthropic"}, 5_900_000, 0, price.StatusUnavailable)
	hot := makeSummary([]tDay{
		{"2026-08-01", 2_000_000}, {"2026-08-02", 2_000_000},
		{"2026-08-03", 2_000_000}, {"2026-08-08", 2_000_000},
	},
		[]tModel{{"a", 4_400_000}, {"b", 3_600_000}},
		[]string{"anthropic"}, 6_100_000, 0, price.StatusUnavailable)
	pWarm := Evaluate(warm, "crossing")
	pHot := Evaluate(hot, "crossing")
	if !inPool("intensity_light", pWarm.Primary) {
		t.Fatalf("73.75%% hit rate should fall back to light, got %q", pWarm.Primary)
	}
	if !inPool("cache_high", pHot.Primary) {
		t.Fatalf("76.25%% hit rate should render cache reuse, got %q", pHot.Primary)
	}
}

// (f) Empty and tiny windows are explicit states, never a phrase.
func TestEvaluateNoneAndInsufficient(t *testing.T) {
	none := Evaluate(metric.Summary{}, "seed")
	if none.State != StateNone || none.Primary != "—" {
		t.Fatalf("none: %+v", none)
	}
	if len(none.Tags) != 0 || none.Detail != "" {
		t.Fatalf("none must carry no phrasing: %+v", none)
	}
	tiny := Evaluate(metric.Summary{All: metric.Slice{ID: "all", Miss: 50_000}}, "seed")
	if tiny.State != StateInsufficient || tiny.Primary != "数据不足" {
		t.Fatalf("insufficient: %+v", tiny)
	}
	if len(tiny.Tags) != 0 || tiny.Detail != "" {
		t.Fatalf("insufficient must carry no phrasing: %+v", tiny)
	}
}
