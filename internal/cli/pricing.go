package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/table"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

// `wheretoken pricing` prints the baked-in list-price card with its official
// sources. It reads the same table the cost calculator uses — there is no
// second, drift-prone copy.

type pricingModelJSON struct {
	Model           string   `json:"model"`
	Input           float64  `json:"input"`
	CacheRead       *float64 `json:"cache_read"`
	CacheCreate     *float64 `json:"cache_create"`
	CacheCreateFree bool     `json:"cache_create_free,omitempty"`
	Output          float64  `json:"output"`
}

type pricingVendorJSON struct {
	Vendor   string             `json:"vendor"`
	Label    string             `json:"label"`
	Source   string             `json:"source"`
	Verified string             `json:"verified"`
	Models   []pricingModelJSON `json:"models"`
}

type pricingJSON struct {
	Card      string              `json:"card"`
	Unit      string              `json:"unit"`
	Providers []pricingVendorJSON `json:"providers"`
}

type pricingGroup struct {
	meta price.SourceMeta
	rows []price.Rate
}

// collectPricing groups the card by vendor, keeping table order, after
// applying the --vendor / --model filters.
func collectPricing(vend, model string) []pricingGroup {
	q := price.Canonical(model)
	var out []pricingGroup
	idx := map[string]int{}
	for _, r := range price.Rates() {
		if vend != "" && r.Vendor != vend {
			continue
		}
		if q != "" && !strings.Contains(r.Model, q) && !price.MatchModel(q, r.Model) {
			continue
		}
		i, ok := idx[r.Vendor]
		if !ok {
			meta, _ := price.SourceFor(r.Source)
			meta.Vendor = r.Vendor
			out = append(out, pricingGroup{meta: meta})
			i = len(out) - 1
			idx[r.Vendor] = i
		}
		out[i].rows = append(out[i].rows, r)
	}
	return out
}

// formatRate prints a USD/1M rate with 2–4 decimals: $5.00, $0.50, $0.0750
// stays exact, never scientific notation.
func formatRate(v float64) string {
	s := strconv.FormatFloat(v, 'f', 4, 64)
	s = strings.TrimRight(s, "0")
	i := strings.IndexByte(s, '.')
	for dec := len(s) - i - 1; dec < 2; dec++ {
		s += "0"
	}
	return "$" + s
}

// rateCell renders one rate component. An unlisted component is "—"
// (unknown, not $0); a card that lists cache write as free shows $0.00 限免.
func rateCell(usd float64, free bool) string {
	if usd > 0 {
		return formatRate(usd)
	}
	if free {
		return formatRate(0) + " 限免"
	}
	return "—"
}

// ratePtr renders an unlisted component as null; a card that lists cache
// write as free keeps the real 0 so free is never confused with unknown.
func ratePtr(v float64) *float64 {
	if v <= 0 {
		return nil
	}
	f := v
	return &f
}

func ratePtrFree(v float64, free bool) *float64 {
	if v > 0 {
		return &v
	}
	if free {
		z := float64(0)
		return &z
	}
	return nil
}

func (a *App) runPricing(flags Flags) int {
	groups := collectPricing(flags.Vendor, flags.Model)
	if flags.JSON {
		payload := pricingJSON{
			Card:      price.CardVersion,
			Unit:      "usd_per_1m_tokens",
			Providers: []pricingVendorJSON{},
		}
		for _, g := range groups {
			pv := pricingVendorJSON{
				Vendor:   g.meta.Vendor,
				Label:    vendor.Label(g.meta.Vendor),
				Source:   g.meta.URL,
				Verified: g.meta.Verified,
				Models:   []pricingModelJSON{},
			}
			for _, r := range g.rows {
				pv.Models = append(pv.Models, pricingModelJSON{
					Model:           r.Model,
					Input:           r.Miss,
					CacheRead:       ratePtr(r.CacheRead),
					CacheCreate:     ratePtrFree(r.CacheCreate, r.CreateFree),
					CacheCreateFree: r.CacheCreate <= 0 && r.CreateFree,
					Output:          r.Output,
				})
			}
			payload.Providers = append(payload.Providers, pv)
		}
		enc := json.NewEncoder(a.Stdout)
		enc.SetIndent("", "  ")
		if err := enc.Encode(payload); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}

	if len(groups) == 0 {
		fmt.Fprintln(a.Stdout, "没有匹配的价目行：该厂家未收录或该模型无公开价（无公开价 ≠ $0）。")
		return ExitOK
	}
	ascii := table.UseASCII(flags.ASCII, a.GOOS, a.LookupEnv)
	color := table.UseColor(flags.NoColor, a.StdoutTTY, a.LookupEnv)
	style := table.BoxUnicode
	if ascii {
		style = table.BoxASCII
	}
	width := resolveWidth(flags.Width, a.LookupEnv, a.termWidth)

	var b strings.Builder
	fmt.Fprintf(&b, "whereToken pricing · 价目卡 %s · 单位 USD / 1M tokens\n", price.CardVersion)
	for _, g := range groups {
		fmt.Fprintf(&b, "\n%s（%s）\n", vendor.Label(g.meta.Vendor), g.meta.Vendor)
		fmt.Fprintf(&b, "来源 %s · 核验 %s\n", g.meta.URL, g.meta.Verified)
		headers := []string{"模型", "输入", "缓存读", "缓存写", "输出"}
		align := []table.Align{table.AlignLeft, table.AlignRight, table.AlignRight, table.AlignRight, table.AlignRight}
		if color {
			for i, h := range headers {
				headers[i] = table.Lemon(h, true)
			}
		}
		body := make([][]string, 0, len(g.rows))
		for _, r := range g.rows {
			body = append(body, []string{
				r.Model,
				rateCell(r.Miss, false),
				rateCell(r.CacheRead, false),
				rateCell(r.CacheCreate, r.CreateFree),
				rateCell(r.Output, false),
			})
		}
		b.WriteString(table.FitRankedTable(headers, body, align, style, width))
	}
	b.WriteString("\n— = 该档无公开价（不会当成 $0）；限免 = 官方列出限时免费。\n")
	b.WriteString("估价为 API 标价等价，不是订阅账单；核验日期是维护者核验日，不是运行日期。\n")
	fmt.Fprint(a.Stdout, b.String())
	return ExitOK
}
