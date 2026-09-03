package cli

import (
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
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
	// Usage is set only by `pricing --usage --json`; plain --json keeps the
	// original card-only shape.
	Usage *usageJSON `json:"usage,omitempty"`
}

// usageJSON is the additive `pricing --usage --json` block: the local
// ledger's per-model usage priced against the same public card.
type usageJSON struct {
	Period  string            `json:"period"`
	Vendors []usageVendorJSON `json:"vendors"`
	Total   usageTotalJSON    `json:"total"`
}

type usageVendorJSON struct {
	Vendor string             `json:"vendor"`
	Label  string             `json:"label"`
	Models []metric.ModelView `json:"models"`
}

type usageTotalJSON struct {
	Total          int64  `json:"total"`
	TotalM         string `json:"total_m"`
	CostStatus     string `json:"cost_status"`
	CostUSD        string `json:"cost_usd,omitempty"`
	UnpricedTokens int64  `json:"unpriced_tokens,omitempty"`
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

func (a *App) runPricing(flags Flags, home adapter.Home) int {
	if flags.Usage {
		return a.runPricingUsage(flags, home)
	}
	groups := collectPricing(flags.Vendor, flags.Model)
	if flags.JSON {
		if err := a.writePricingJSON(catalogPayload(groups)); err != nil {
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

// catalogPayload is the plain `pricing --json` card dump. `pricing --usage
// --json` reuses it and only adds the usage block on top.
func catalogPayload(groups []pricingGroup) pricingJSON {
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
	return payload
}

func (a *App) writePricingJSON(payload pricingJSON) error {
	enc := json.NewEncoder(a.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(payload)
}

// runPricingUsage scans local ledgers exactly like the report does, then
// prices every normalized model against the same public card the catalog
// prints. Unpriced usage stays unavailable — it is never rewritten as $0.
func (a *App) runPricingUsage(flags Flags, home adapter.Home) int {
	res := a.doScan(home, flags.Quiet, flags.Offline, flags.ASCII)
	win, err := metric.ParseWindow(flags.Today, flags.Since, flags.From, flags.To, a.Now(), a.Loc)
	if err != nil {
		fmt.Fprintln(a.Stderr, err.Error())
		return ExitUsage
	}
	var events []event.UsageEvent
	for _, e := range res.Events {
		if win.Contains(e.Timestamp, a.Loc) {
			events = append(events, e)
		}
	}
	sum := metric.AggregateAt(events, nil, a.Now(), a.Loc)
	rows := filterUsageModels(sum.ByModel, flags.Vendor, flags.Model)
	if flags.JSON {
		payload := catalogPayload(collectPricing(flags.Vendor, flags.Model))
		payload.Usage = usagePayload(rows, win.Label)
		if err := a.writePricingJSON(payload); err != nil {
			fmt.Fprintln(a.Stderr, err.Error())
			return ExitFail
		}
		return ExitOK
	}
	a.printUsageBreakdown(flags, rows, win.Label)
	return ExitOK
}

// filterUsageModels keeps the --vendor / --model slice. A model query matches
// the normalized ledger id the same fuzzy way the catalog filter matches card
// rows; the unknown-model bucket matches no query.
func filterUsageModels(rows []metric.ModelSlice, vend, model string) []metric.ModelSlice {
	q := price.Canonical(model)
	var out []metric.ModelSlice
	for _, m := range rows {
		if vend != "" && m.Vendor != vend {
			continue
		}
		if q != "" && (m.Model == "" || (!strings.Contains(m.Model, q) && !price.MatchModel(q, m.Model))) {
			continue
		}
		out = append(out, m)
	}
	return out
}

type usageVendorGroup struct {
	id   string
	rows []metric.ModelSlice
}

// groupUsageByVendor folds the globally total-sorted ByModel rows into vendor
// sections, keeping first-appearance order (heaviest vendor first).
func groupUsageByVendor(rows []metric.ModelSlice) []usageVendorGroup {
	var out []usageVendorGroup
	idx := map[string]int{}
	for _, m := range rows {
		i, ok := idx[m.Vendor]
		if !ok {
			out = append(out, usageVendorGroup{id: m.Vendor})
			i = len(out) - 1
			idx[m.Vendor] = i
		}
		out[i].rows = append(out[i].rows, m)
	}
	return out
}

// usageTotals accumulates the 总计 row. Summed over every model row it equals
// the dashboard estimate for the window; unpriced tokens are tracked so the
// total never looks complete when part of the usage has no public price.
type usageTotals struct {
	tokens, priced, unpriced, micro int64
}

func (t *usageTotals) add(m metric.ModelSlice) {
	t.tokens += m.Slice.Total()
	t.priced += m.Slice.PricedTokens
	t.unpriced += m.Slice.UnpricedTokens
	t.micro += m.Slice.CostMicro
}

func (t usageTotals) status() string { return price.Status(t.priced, t.unpriced) }

// costText renders a cost with the unavailable semantics the whole CLI
// shares: a real price, or — plus the reason when tokens have no public price.
func costText(status string, micro, unpriced int64) string {
	usd := metric.FormatCostUSD(status, micro)
	if unpriced > 0 {
		if usd == "" {
			return "— · 部分用量无公开价"
		}
		return usd + " · 部分用量无公开价"
	}
	if usd == "" {
		return "—"
	}
	return usd
}

func usagePayload(rows []metric.ModelSlice, period string) *usageJSON {
	u := &usageJSON{Period: period, Vendors: []usageVendorJSON{}}
	var tot usageTotals
	for _, g := range groupUsageByVendor(rows) {
		vg := usageVendorJSON{Vendor: g.id, Label: vendor.Label(g.id), Models: []metric.ModelView{}}
		for _, m := range g.rows {
			tot.add(m)
			vg.Models = append(vg.Models, metric.ViewModel(m))
		}
		u.Vendors = append(u.Vendors, vg)
	}
	u.Total = usageTotalJSON{
		Total:          tot.tokens,
		TotalM:         metric.FormatM(tot.tokens),
		CostStatus:     tot.status(),
		CostUSD:        metric.FormatCostUSD(tot.status(), tot.micro),
		UnpricedTokens: tot.unpriced,
	}
	return u
}

func (a *App) printUsageBreakdown(flags Flags, rows []metric.ModelSlice, period string) {
	ascii := table.UseASCII(flags.ASCII, a.GOOS, a.LookupEnv)
	color := table.UseColor(flags.NoColor, a.StdoutTTY, a.LookupEnv)
	style := table.BoxUnicode
	if ascii {
		style = table.BoxASCII
	}
	width := resolveWidth(flags.Width, a.LookupEnv, a.termWidth)

	var b strings.Builder
	b.WriteString("单价单位：美元 / 每百万 tokens（USD per 1M tokens）\n")
	fmt.Fprintf(&b, "周期：%s\n", period)
	if len(rows) == 0 {
		b.WriteString("\n该范围没有可估价的用量。\n")
		fmt.Fprint(a.Stdout, b.String())
		return
	}
	var tot usageTotals
	for _, g := range groupUsageByVendor(rows) {
		fmt.Fprintf(&b, "\n%s（%s）\n", vendor.Label(g.id), g.id)
		headers := []string{"模型", "未命中", "缓存读", "缓存写", "输出", "估价"}
		align := []table.Align{table.AlignLeft, table.AlignRight, table.AlignRight, table.AlignRight, table.AlignRight, table.AlignRight}
		if color {
			for i, h := range headers {
				headers[i] = table.Lemon(h, true)
			}
		}
		body := make([][]string, 0, len(g.rows))
		for _, m := range g.rows {
			tot.add(m)
			body = append(body, usageModelRow(m))
		}
		b.WriteString(table.FitRankedTable(headers, body, align, style, width))
	}
	fmt.Fprintf(&b, "\n总计 %s · 估价 %s\n", metric.FormatM(tot.tokens), costText(tot.status(), tot.micro, tot.unpriced))
	b.WriteString("单元格 = 用量 × 单价；— = 该档无用量或无公开价（无公开价 ≠ $0）；限免 = 官方列出限时免费。\n")
	b.WriteString("估价为 API 标价等价，不是订阅账单；只计入有公开价的用量。\n")
	fmt.Fprint(a.Stdout, b.String())
}

func usageModelRow(m metric.ModelSlice) []string {
	var miss, cacheRead, cacheCreate, output float64
	free := false
	if r := m.Rate; r != nil {
		miss, cacheRead, cacheCreate, output = r.Miss, r.CacheRead, r.CacheCreate, r.Output
		free = r.CreateFree
	}
	return []string{
		table.Truncate(m.Slice.Label, 36),
		usageCell(m.Slice.Miss, miss, false),
		usageCell(m.Slice.CacheRead, cacheRead, false),
		usageCell(m.Slice.CacheCreate, cacheCreate, free),
		usageCell(m.Slice.Output, output, false),
		costText(m.Slice.CostStatus, m.Slice.CostMicro, m.Slice.UnpricedTokens),
	}
}

// usageCell pairs the tokens burned in one category with the card rate behind
// them. No tokens is — (nothing burned); tokens without a listed rate end in
// × — (unknown, never $0); a card-listed free component says 限免.
func usageCell(tokens int64, usdPerM float64, free bool) string {
	if tokens <= 0 {
		return "—"
	}
	return metric.FormatM(tokens) + " × " + rateCell(usdPerM, free)
}
