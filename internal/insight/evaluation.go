package insight

import (
	"fmt"
	"math"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

// Evaluation is a deterministic, explainable usage profile computed from the
// same Summary the dashboard renders. No ML, no network, no industry
// percentile: it is a fixed rule set over this user's own window, so it is
// testable and works offline.
type Evaluation struct {
	Level   string `json:"level"`
	Summary string `json:"summary"`
	Reason  string `json:"reason,omitempty"`
}

// Levels, in priority order: the first matching rule wins.
const (
	LevelNone         = "none"         // no records in the window at all
	LevelInsufficient = "insufficient" // some records, too few to profile
	LevelHighCost     = "high_cost"    // API-equivalent cost per active day is high
	LevelHighUsage    = "high_usage"   // tokens per active day are high
	LevelMultiModel   = "multi_model"  // spread across vendors and models
	LevelSingleModel  = "single_model" // one model carries the window
	LevelCacheReuse   = "cache_reuse"  // very high cache-read share at volume
	LevelSteady       = "steady"       // regular moderate usage
	LevelLight        = "light"        // everything else that has data
)

// Rule thresholds. Token volumes mix cached and uncached traffic, so
// intensity is normalized per active day instead of pretending there is a
// universal "heavy user" token count.
const (
	evalMinTokens         int64   = 100_000   // below this a profile is noise
	highCostPerDayUSD     float64 = 25        // API-equivalent list cost
	highUsagePerDayTokens int64   = 5_000_000 // tokens per active day

	multiModelMinVendors    = 3
	multiModelMinModels     = 3
	multiModelMaxTopVendor  = 60 // percent of window tokens
	singleModelMinShare     = 70 // percent of window tokens
	cacheReuseMinHitPct     = 75 // cache_read / (read+miss+create)
	cacheReuseMinReadTokens = 5_000_000

	steadyMinActiveDays     = 4
	steadyMinPerDayTokens   = 300_000
	insufficientSummaryText = "暂无评价"
)

// Evaluate profiles the window behind sum. Empty data is LevelNone ("—"),
// never "light": no records is not the same as small usage.
func Evaluate(sum metric.Summary) Evaluation {
	tot := sum.All.Total()
	if tot <= 0 {
		return Evaluation{Level: LevelNone, Summary: "—"}
	}
	if tot < evalMinTokens {
		return Evaluation{
			Level:   LevelInsufficient,
			Summary: insufficientSummaryText,
			Reason:  fmt.Sprintf("本周期只有 %s tokens，还看不出使用画像。", metric.FormatM(tot)),
		}
	}
	days := int64(len(sum.Calendar.All.Days))
	if days <= 0 {
		days = 1
	}
	perDay := tot / days

	if usd := metric.FormatCostUSD(sum.All.CostStatus, sum.All.CostMicro); usd != "" {
		perDayCost := float64(sum.All.CostMicro) / 1e6 / float64(days)
		if perDayCost >= highCostPerDayUSD {
			reason := fmt.Sprintf("本周期预计成本 %s，活跃日均约 $%.2f/天", usd, perDayCost)
			if m := topCostModel(sum); m != "" {
				reason += "，主要来自 " + m
			}
			return Evaluation{Level: LevelHighCost, Summary: "成本偏高", Reason: reason + "。"}
		}
	}

	if perDay >= highUsagePerDayTokens {
		reason := fmt.Sprintf("本周期累计 %s tokens，%d 个活跃日", metric.FormatM(tot), days)
		if v := topVendorText(sum, tot); v != "" {
			reason += "，" + v
		}
		return Evaluation{Level: LevelHighUsage, Summary: "高强度使用", Reason: reason + "。"}
	}

	known := knownVendors(sum)
	if len(known) >= multiModelMinVendors && len(labeledModels(sum)) >= multiModelMinModels &&
		!shareAtLeast(known[0].Total(), tot, multiModelMaxTopVendor) {
		return Evaluation{
			Level:   LevelMultiModel,
			Summary: "多模型探索",
			Reason:  fmt.Sprintf("使用覆盖 %d 个厂家、%d 个模型，分布较均衡。", len(known), len(labeledModels(sum))),
		}
	}

	if top := topLabeledModel(sum); top != nil && shareAtLeast(top.Total(), tot, singleModelMinShare) {
		return Evaluation{
			Level:   LevelSingleModel,
			Summary: "单模型集中",
			Reason:  fmt.Sprintf("token 的 %d%% 集中在 %s。", pct(top.Total(), tot), top.Label),
		}
	}

	if hit, ok := metric.HitRate(sum.All.Miss, sum.All.CacheRead, sum.All.CacheCreate); ok &&
		hit >= cacheReuseMinHitPct && sum.All.CacheRead >= cacheReuseMinReadTokens {
		return Evaluation{
			Level:   LevelCacheReuse,
			Summary: "高缓存复用",
			Reason:  fmt.Sprintf("缓存读占输入侧 %.0f%%，上下文复用率很高。", hit),
		}
	}

	if days >= steadyMinActiveDays && perDay >= steadyMinPerDayTokens {
		return Evaluation{
			Level:   LevelSteady,
			Summary: "稳定使用",
			Reason:  fmt.Sprintf("活跃 %d 天，日均 %s tokens，节奏稳定。", days, metric.FormatM(perDay)),
		}
	}

	reason := "本周期调用较少。"
	if sum.All.CostStatus != price.StatusUnavailable && metric.FormatCostUSD(sum.All.CostStatus, sum.All.CostMicro) != "" {
		reason = "本周期调用较少，预计成本较低。"
	}
	return Evaluation{Level: LevelLight, Summary: "轻量使用", Reason: reason}
}

// shareAtLeast reports part/all >= pct% using integer math so a boundary
// share (exactly 70%) lands inside the rule without float rounding drift.
func shareAtLeast(part, all, pct int64) bool {
	if part <= 0 || all <= 0 {
		return false
	}
	if all <= math.MaxInt64/100 && part <= math.MaxInt64/100 {
		return part*100 >= pct*all
	}
	return float64(part)/float64(all)*100 >= float64(pct)
}

func knownVendors(sum metric.Summary) []metric.Slice {
	var out []metric.Slice
	for _, s := range sum.ByVendor {
		if s.ID == "unknown" || s.Total() <= 0 {
			continue
		}
		out = append(out, s)
	}
	return out
}

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

func topLabeledModel(sum metric.Summary) *metric.Slice {
	models := labeledModels(sum)
	if len(models) == 0 {
		return nil
	}
	return &models[0]
}

func topCostModel(sum metric.Summary) string {
	best := int64(0)
	label := ""
	for _, s := range sum.DrillAll.Models {
		if metric.UnlabeledDrillID(s.ID) || s.CostMicro <= best {
			continue
		}
		best = s.CostMicro
		label = s.Label
	}
	return label
}

// topVendorText names the one or two vendors most of the window ran on.
func topVendorText(sum metric.Summary, tot int64) string {
	known := knownVendors(sum)
	if len(known) == 0 {
		return ""
	}
	name := func(s metric.Slice) string { return vendor.Label(s.ID) }
	if len(known) == 1 || !shareAtLeast(known[1].Total(), tot, 10) {
		return "主要来自 " + name(known[0])
	}
	return fmt.Sprintf("主要集中在 %s 与 %s", name(known[0]), name(known[1]))
}
