// Package insight builds deterministic, re-derivable usage lines from a
// Summary. No ML, no network, no guessed numbers.
package insight

import (
	"fmt"

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
			Text: fmt.Sprintf("%s · %d%% of tokens", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.ByVendor) > 0 && sum.ByVendor[0].Total() > 0 && sum.ByVendor[0].ID != "unknown" {
		s := sum.ByVendor[0]
		out = append(out, Line{
			Kind: "largest_vendor",
			Text: fmt.Sprintf("%s · %d%% of tokens", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.DrillAll.Models) > 0 && sum.DrillAll.Models[0].Total() > 0 {
		s := sum.DrillAll.Models[0]
		out = append(out, Line{
			Kind: "largest_model",
			Text: fmt.Sprintf("%s · %d%% of tokens", s.Label, pct(s.Total(), tot)),
		})
	}
	if len(sum.DrillAll.Sessions) > 0 && sum.DrillAll.Sessions[0].Total() > 0 {
		s := sum.DrillAll.Sessions[0]
		out = append(out, Line{
			Kind: "largest_session",
			Text: fmt.Sprintf("session %s · %d%% of tokens", s.Label, pct(s.Total(), tot)),
		})
	}
	in := sum.All.Miss + sum.All.CacheRead + sum.All.CacheCreate
	if in > 0 && sum.All.CacheRead > 0 {
		out = append(out, Line{
			Kind: "cache",
			Text: fmt.Sprintf("Cache Read · %d%% of input-side tokens", pct(sum.All.CacheRead, in)),
		})
	}
	if sum.All.CostStatus == price.StatusComplete || sum.All.CostStatus == price.StatusPartial {
		if sum.All.CostMicro > 0 {
			out = append(out, Line{
				Kind: "cost",
				Text: fmt.Sprintf("API-equivalent · %s (%s)", price.FormatUSD(sum.All.CostMicro), sum.All.CostStatus),
			})
		}
	}
	if sum.All.CostStatus == price.StatusPartial && sum.All.UnpricedTokens > 0 {
		out = append(out, Line{
			Kind: "unpriced",
			Text: fmt.Sprintf("Unpriced · %s tokens have no list price (not $0)", metric.FormatM(sum.All.UnpricedTokens)),
		})
	}
	if sum.All.CostStatus == price.StatusUnavailable && tot > 0 {
		out = append(out, Line{
			Kind: "unpriced",
			Text: "Estimated API-equivalent cost unavailable · missing price is not written as zero",
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
