// Package insight builds deterministic, re-derivable usage lines from a
// Summary. No ML, no network, no guessed numbers.
package insight

import (
	"fmt"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

type Line struct {
	Kind string `json:"kind"`
	Text string `json:"text"`
}

func Lines(sum metric.Summary) []Line {
	var out []Line
	tot := sum.All.Total()
	if tot <= 0 {
		return out
	}
	if len(sum.BySource) > 0 && sum.BySource[0].Total() > 0 {
		s := sum.BySource[0]
		out = append(out, Line{
			Kind: "largest_tool",
			Text: fmt.Sprintf("%s · 占 token %d%%", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.ByVendor) > 0 && sum.ByVendor[0].Total() > 0 && sum.ByVendor[0].ID != "unknown" {
		s := sum.ByVendor[0]
		out = append(out, Line{
			Kind: "largest_vendor",
			Text: fmt.Sprintf("%s · 占 token %d%%", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.DrillAll.Models) > 0 && sum.DrillAll.Models[0].Total() > 0 && !metric.UnlabeledDrillID(sum.DrillAll.Models[0].ID) {
		s := sum.DrillAll.Models[0]
		out = append(out, Line{
			Kind: "largest_model",
			Text: fmt.Sprintf("%s · 占 token %d%%", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.DrillAll.Sessions) > 0 && sum.DrillAll.Sessions[0].Total() > 0 && !metric.UnlabeledDrillID(sum.DrillAll.Sessions[0].ID) {
		s := sum.DrillAll.Sessions[0]
		out = append(out, Line{
			Kind: "largest_session",
			Text: fmt.Sprintf("会话 %s · 占 token %d%%", s.Label, pct(s.Total(), tot)),
		})
	}
	in := sum.All.Miss + sum.All.CacheRead + sum.All.CacheCreate
	if in > 0 && sum.All.CacheRead > 0 {
		out = append(out, Line{
			Kind: "cache",
			Text: fmt.Sprintf("缓存读 · 占输入侧 token %d%%", pct(sum.All.CacheRead, in)),
		})
	}
	if sum.All.CostStatus == price.StatusComplete || sum.All.CostStatus == price.StatusPartial {
		if sum.All.CostMicro > 0 {
			out = append(out, Line{
				Kind: "cost",
				Text: fmt.Sprintf("API 标价等价 · %s（%s）", price.FormatUSD(sum.All.CostMicro), sum.All.CostStatus),
			})
		}
	}
	if sum.All.CostStatus == price.StatusPartial && sum.All.UnpricedTokens > 0 {
		out = append(out, Line{
			Kind: "unpriced",
			Text: fmt.Sprintf("无标价 · %s token 没有公开标价，不会写成 $0", metric.FormatM(sum.All.UnpricedTokens)),
		})
	}
	if sum.All.CostStatus == price.StatusUnavailable && tot > 0 {
		out = append(out, Line{
			Kind: "unpriced",
			Text: "估价不可用 · 没有标价不会写成 0",
		})
	}
	return out
}

func pct(part, all int64) int64 {
	if all <= 0 {
		return 0
	}
	return (part*100 + all/2) / all
}

// AppendStanding adds a Community Rank line only when the standing is a real
// place. Unknown rank is omitted, never written as #0 or a global podium.
func AppendStanding(lines []Line, status, display string, rank int) []Line {
	if status != "ok" || rank <= 0 || display == "" {
		return lines
	}
	if strings.Contains(display, "#0 /") || strings.Contains(display, "#0/") || display == "#0" {
		return lines
	}
	return append(lines, Line{
		Kind: "community",
		Text: "社区排名 " + display + " · 匿名聚合，不是全球榜",
	})
}
