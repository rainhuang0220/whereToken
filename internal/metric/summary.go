package metric

import (
	"fmt"
	"math"
	"sort"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Slice struct {
	ID, Label                             string
	Miss, CacheRead, CacheCreate, Output  int64
	Requests, UserTurns                   int64
	Records                               int64
	Derivation                            string
	Quality                               event.Quality
	CostMicro                             int64
	MissCostMicro, CacheReadCostMicro     int64
	CacheCreateCostMicro, OutputCostMicro int64
	PricedTokens, UnpricedTokens          int64
	CostStatus                            string
}

func (s Slice) Total() int64 {
	return satAdd(satAdd(s.Miss, s.CacheRead), satAdd(s.CacheCreate, s.Output))
}

type SourceVendor struct {
	Source, Vendor, SourceLabel, VendorLabel string
	Miss, CacheRead, CacheCreate, Output     int64
	Requests                                 int64
	CostMicro                                int64
	PricedTokens, UnpricedTokens             int64
	CostStatus                               string
}

func (s SourceVendor) Total() int64 {
	return satAdd(satAdd(s.Miss, s.CacheRead), satAdd(s.CacheCreate, s.Output))
}

type Summary struct {
	All            Slice
	BySource       []Slice
	ByVendor       []Slice
	BySourceVendor []SourceVendor
	Calendar       Calendar
	DrillAll       DrillPack
	DrillBySource  map[string]DrillPack
	DrillByVendor  map[string]DrillPack
}

type SliceView struct {
	ID                 string   `json:"id"`
	Label              string   `json:"label"`
	Miss               int64    `json:"miss"`
	CacheRead          int64    `json:"cache_read"`
	CacheCreate        int64    `json:"cache_create"`
	Output             int64    `json:"output"`
	Total              int64    `json:"total"`
	MissM              string   `json:"miss_m"`
	CacheReadM         string   `json:"cache_read_m"`
	CacheCreateM       string   `json:"cache_create_m"`
	OutputM            string   `json:"output_m"`
	TotalM             string   `json:"total_m"`
	HitRate            *float64 `json:"hit_rate"`
	HitRateText        string   `json:"hit_rate_text"`
	Requests           int64    `json:"requests"`
	UserTurns          int64    `json:"user_turns"`
	Records            int64    `json:"records,omitempty"`
	Derivation         string   `json:"derivation,omitempty"`
	Quality            string   `json:"quality"`
	Error              string   `json:"error,omitempty"`
	CostStatus         string   `json:"cost_status,omitempty"`
	CostUSD            string   `json:"cost_usd,omitempty"`
	MissCostUSD        string   `json:"miss_cost_usd,omitempty"`
	CacheReadCostUSD   string   `json:"cache_read_cost_usd,omitempty"`
	CacheCreateCostUSD string   `json:"cache_create_cost_usd,omitempty"`
	OutputCostUSD      string   `json:"output_cost_usd,omitempty"`
	UnpricedTokens     int64    `json:"unpriced_tokens,omitempty"`
}

func View(s Slice) SliceView {
	v := SliceView{
		ID:           s.ID,
		Label:        s.Label,
		Miss:         s.Miss,
		CacheRead:    s.CacheRead,
		CacheCreate:  s.CacheCreate,
		Output:       s.Output,
		Total:        s.Total(),
		MissM:        FormatM(s.Miss),
		CacheReadM:   FormatM(s.CacheRead),
		CacheCreateM: FormatM(s.CacheCreate),
		OutputM:      FormatM(s.Output),
		TotalM:       FormatM(s.Total()),
		HitRateText:  "—",
		Requests:     s.Requests,
		UserTurns:    s.UserTurns,
		Records:      s.Records,
		Derivation:   s.Derivation,
		Quality:      string(s.Quality),
	}
	st := s.CostStatus
	if st == "" {
		st = price.Status(s.PricedTokens, s.UnpricedTokens)
	}
	v.CostStatus = st
	if usd := FormatCostUSD(st, s.CostMicro); usd != "" {
		v.CostUSD = usd
		v.MissCostUSD = formatCostPart(s.MissCostMicro)
		v.CacheReadCostUSD = formatCostPart(s.CacheReadCostMicro)
		v.CacheCreateCostUSD = formatCostPart(s.CacheCreateCostMicro)
		v.OutputCostUSD = formatCostPart(s.OutputCostMicro)
	}
	v.UnpricedTokens = s.UnpricedTokens
	if pct, ok := HitRate(s.Miss, s.CacheRead, s.CacheCreate); ok {
		v.HitRate = &pct
		v.HitRateText = fmt.Sprintf("%.1f%%", pct)
	}
	return v
}

func FormatCostUSD(status string, micro int64) string {
	if status != price.StatusComplete && !(status == price.StatusPartial && micro > 0) {
		return ""
	}
	return formatCostPart(micro)
}

func formatCostPart(micro int64) string {
	if micro <= 0 {
		return ""
	}
	return price.FormatUSD(micro)
}

// CostSlice merges and prices events without building calendar or drill.
func CostSlice(events []event.UsageEvent) Slice {
	merged := mergeByRequest(events)
	all := Slice{ID: "all", Label: "合计"}
	for _, e := range merged {
		addSlice(&all, e)
	}
	finishCost(&all)
	return all
}

func Aggregate(events []event.UsageEvent, turns []event.TurnEvent) Summary {
	return AggregateAt(events, turns, time.Now(), time.Local)
}

// AggregateAt is Aggregate with an explicit clock for the calendar window.
func AggregateAt(events []event.UsageEvent, turns []event.TurnEvent, now time.Time, loc *time.Location) Summary {
	merged := mergeByRequest(events)

	all := Slice{ID: "all", Label: "合计"}
	bySource := map[string]*Slice{}
	byVendor := map[string]*Slice{}
	byCross := map[string]*SourceVendor{}
	srcDerive := map[string]map[string]struct{}{}
	allDerive := map[string]struct{}{}

	for _, e := range merged {
		addSlice(&all, e)
		src := getSlice(bySource, e.Source, sourceLabel(e.Source))
		addSlice(src, e)
		src.Records++
		if e.Derivation != "" {
			if srcDerive[e.Source] == nil {
				srcDerive[e.Source] = map[string]struct{}{}
			}
			srcDerive[e.Source][e.Derivation] = struct{}{}
			allDerive[e.Derivation] = struct{}{}
		}
		vend := getSlice(byVendor, e.Vendor, vendor.Label(e.Vendor))
		addSlice(vend, e)
		key := e.Source + "\x00" + e.Vendor
		cross := byCross[key]
		if cross == nil {
			cross = &SourceVendor{
				Source:      e.Source,
				Vendor:      e.Vendor,
				SourceLabel: sourceLabel(e.Source),
				VendorLabel: vendor.Label(e.Vendor),
			}
			byCross[key] = cross
		}
		cross.Miss = satAdd(cross.Miss, e.Miss)
		cross.CacheRead = satAdd(cross.CacheRead, e.CacheRead)
		cross.CacheCreate = satAdd(cross.CacheCreate, e.CacheCreate)
		cross.Output = satAdd(cross.Output, e.Output)
		if !e.SkipRequest {
			cross.Requests++
		}
		toks := satAdd(satAdd(e.Miss, e.CacheRead), satAdd(e.CacheCreate, e.Output))
		ch := price.Event(e)
		if ch.OK {
			cross.CostMicro = satAdd(cross.CostMicro, ch.Micro)
			cross.PricedTokens = satAdd(cross.PricedTokens, toks)
		} else if toks > 0 {
			cross.UnpricedTokens = satAdd(cross.UnpricedTokens, toks)
		}
	}

	for _, t := range turns {
		all.UserTurns++
		src := getSlice(bySource, t.Source, sourceLabel(t.Source))
		src.UserTurns++
	}

	all.Records = int64(len(merged))
	all.Derivation = joinKeys(allDerive)
	finishCost(&all)
	sum := Summary{All: all}
	for _, s := range bySource {
		s.Derivation = joinKeys(srcDerive[s.ID])
		finishCost(s)
		sum.BySource = append(sum.BySource, *s)
	}
	for _, s := range byVendor {
		finishCost(s)
		sum.ByVendor = append(sum.ByVendor, *s)
	}
	for _, s := range byCross {
		s.CostStatus = price.Status(s.PricedTokens, s.UnpricedTokens)
		sum.BySourceVendor = append(sum.BySourceVendor, *s)
	}
	sort.Slice(sum.BySource, func(i, j int) bool { return sum.BySource[i].Total() > sum.BySource[j].Total() })
	sort.Slice(sum.ByVendor, func(i, j int) bool { return sum.ByVendor[i].Total() > sum.ByVendor[j].Total() })
	sort.Slice(sum.BySourceVendor, func(i, j int) bool { return sum.BySourceVendor[i].Total() > sum.BySourceVendor[j].Total() })
	sum.Calendar = BuildCalendar(merged, loc, now)
	sum.DrillAll, sum.DrillBySource, sum.DrillByVendor = buildDrill(merged, turns)
	return sum
}

func usableTokens(e event.UsageEvent) bool {
	return e.Miss >= 0 && e.CacheRead >= 0 && e.CacheCreate >= 0 && e.Output >= 0
}

func satAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

func mergeByRequest(events []event.UsageEvent) []event.UsageEvent {
	var out []event.UsageEvent
	index := map[string]int{}
	for _, e := range events {
		if !usableTokens(e) {
			continue
		}
		if e.RequestID == "" {
			out = append(out, e)
			continue
		}
		key := e.Source + "\x00" + e.RequestID
		if i, ok := index[key]; ok {
			out[i] = maxEvent(out[i], e)
			continue
		}
		index[key] = len(out)
		out = append(out, e)
	}
	return out
}

func maxEvent(a, b event.UsageEvent) event.UsageEvent {
	if b.Miss > a.Miss {
		a.Miss = b.Miss
	}
	if b.CacheRead > a.CacheRead {
		a.CacheRead = b.CacheRead
	}
	if b.CacheCreate > a.CacheCreate {
		a.CacheCreate = b.CacheCreate
	}
	if b.Output > a.Output {
		a.Output = b.Output
	}
	if b.Reasoning > a.Reasoning {
		a.Reasoning = b.Reasoning
	}
	if qualityRank(b.Quality) > qualityRank(a.Quality) {
		a.Quality = b.Quality
	}
	if a.Timestamp.IsZero() || (!b.Timestamp.IsZero() && b.Timestamp.After(a.Timestamp)) {
		a.Timestamp = b.Timestamp
	}
	return a
}

func qualityRank(q event.Quality) int {
	switch q {
	case event.QualityDegraded:
		return 3
	case event.QualityEstimated:
		return 2
	case event.QualityAuthoritative:
		return 1
	default:
		return 0
	}
}

func addSlice(s *Slice, e event.UsageEvent) {
	s.Miss = satAdd(s.Miss, e.Miss)
	s.CacheRead = satAdd(s.CacheRead, e.CacheRead)
	s.CacheCreate = satAdd(s.CacheCreate, e.CacheCreate)
	s.Output = satAdd(s.Output, e.Output)
	if !e.SkipRequest {
		s.Requests++
	}
	if qualityRank(e.Quality) > qualityRank(s.Quality) {
		s.Quality = e.Quality
	}
	toks := satAdd(satAdd(e.Miss, e.CacheRead), satAdd(e.CacheCreate, e.Output))
	ch := price.Event(e)
	if ch.OK {
		s.CostMicro = satAdd(s.CostMicro, ch.Micro)
		s.MissCostMicro = satAdd(s.MissCostMicro, ch.Miss)
		s.CacheReadCostMicro = satAdd(s.CacheReadCostMicro, ch.CacheRead)
		s.CacheCreateCostMicro = satAdd(s.CacheCreateCostMicro, ch.CacheCreate)
		s.OutputCostMicro = satAdd(s.OutputCostMicro, ch.Output)
		s.PricedTokens = satAdd(s.PricedTokens, toks)
		return
	}
	if toks > 0 {
		s.UnpricedTokens = satAdd(s.UnpricedTokens, toks)
	}
}

func finishCost(s *Slice) {
	s.CostStatus = price.Status(s.PricedTokens, s.UnpricedTokens)
}

func getSlice(m map[string]*Slice, id, label string) *Slice {
	if s, ok := m[id]; ok {
		return s
	}
	s := &Slice{ID: id, Label: label}
	m[id] = s
	return s
}

func sourceLabel(id string) string { return adapter.Label(id) }

func SourceLabel(id string) string { return sourceLabel(id) }

func KnownSourceIDs() []string { return adapter.KnownIDs() }

func LookupSource(name string) (string, bool) {
	n := compactName(name)
	if n == "" {
		return "", false
	}
	for _, id := range KnownSourceIDs() {
		if n == compactName(id) || n == compactName(sourceLabel(id)) {
			return id, true
		}
	}
	return "", false
}

func joinKeys(m map[string]struct{}) string {
	if len(m) == 0 {
		return ""
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return strings.Join(keys, ",")
}

func compactName(s string) string {
	var b []rune
	for _, r := range strings.ToLower(strings.TrimSpace(s)) {
		if r == ' ' || r == '-' || r == '_' {
			continue
		}
		b = append(b, r)
	}
	return string(b)
}
