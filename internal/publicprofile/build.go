package publicprofile

import (
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
	"github.com/rainhuang0220/whereToken/internal/profile"
)

const (
	maxBreakdown = 8
	maxSeries    = 8
)

const (
	defaultProjectURL = "https://github.com/rainhuang0220/whereToken"
	defaultAccounting = "https://github.com/rainhuang0220/whereToken/blob/main/docs/token-accounting.md"
	defaultLivePage   = "https://rainhuang0220.github.io/whereToken/profile/"
)

// Build projects one scan into a public Snapshot. One now/loc, one aggregation
// per period. Renderers must not re-scan.
func Build(in Input) (Snapshot, error) {
	loc := in.Loc
	if loc == nil {
		loc = time.UTC
	}
	now := in.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.In(loc)
	owner, err := sanitizeOwner(in.Owner)
	if err != nil {
		return Snapshot{}, err
	}
	version := in.Version
	if version == "" {
		version = "dev"
	}

	allSum := metric.AggregateAt(in.Events, in.Turns, now, loc)
	status := dataStatus(allSum)
	unavailable := status == StatusUnavailable

	windows := periodWindows(now, loc, allSum.Calendar)
	snap := Snapshot{
		Schema:        SchemaName,
		SchemaVersion: SchemaVersion,
		GeneratedAt:   now.UTC().Format(time.RFC3339),
		AsOfDate:      now.Format("2006-01-02"),
		Producer: Producer{
			Name:                   "wheretoken",
			Version:                version,
			TokenAccountingVersion: TokenAccountingVersion,
		},
		Owner: owner,
		Provenance: Provenance{
			Kind:        "local_sanitized_snapshot",
			RefreshMode: "manual_publish",
			LiveSync:    false,
		},
		DataStatus: status,
		Privacy: Privacy{
			RawEvents:       false,
			ModelsIncluded:  in.IncludeModels,
			CostIncluded:    in.IncludeCost,
			DailyPrecision:  true,
			OmittedSections: omitted(in.IncludeModels, in.IncludeCost),
		},
		Notices: notices(status, in.IncludeCost, allSum),
		Links: Links{
			Project:         defaultProjectURL,
			TokenAccounting: defaultAccounting,
			LivePage:        defaultLivePage,
		},
	}

	snap.Periods.All = projectPeriod(PeriodAll, windows[PeriodAll], in, now, loc, status, allSum)
	snap.Periods.Today = projectPeriod(PeriodToday, windows[PeriodToday], in, now, loc, status, metric.Summary{})
	snap.Periods.D7 = projectPeriod(Period7d, windows[Period7d], in, now, loc, status, metric.Summary{})
	snap.Periods.D30 = projectPeriod(Period30d, windows[Period30d], in, now, loc, status, metric.Summary{})
	snap.Periods.W53 = projectPeriod(Period53w, windows[Period53w], in, now, loc, status, metric.Summary{})

	dates := wallDates(allSum.Calendar.WindowFrom, allSum.Calendar.WindowTo)
	snap.Activity = Activity{
		WeekStart: "monday",
		From:      allSum.Calendar.WindowFrom,
		To:        allSum.Calendar.WindowTo,
		Dates:     dates,
		Series:    buildSeries(allSum, dates, allSum.Calendar.WindowTo, unavailable, in.IncludeModels),
	}
	_ = unavailable
	return snap, nil
}

func omitted(models, cost bool) []string {
	var out []string
	if !models {
		out = append(out, "by_model")
	}
	if !cost {
		out = append(out, "cost")
	}
	if out == nil {
		return []string{}
	}
	return out
}

func notices(status string, includeCost bool, sum metric.Summary) []string {
	var out []string
	if status == StatusPartial {
		out = append(out, NoticePartialUsage)
	}
	if includeCost && sum.All.CostStatus == price.StatusPartial {
		out = append(out, NoticeUnpricedCost)
	}
	if out == nil {
		return []string{}
	}
	return out
}

func dataStatus(sum metric.Summary) string {
	if sum.All.Total() == 0 && sum.All.Requests == 0 && sum.All.UserTurns == 0 {
		return StatusUnavailable
	}
	if sum.All.Quality == event.QualityDegraded || sum.All.Quality == event.QualityEstimated {
		return StatusPartial
	}
	for _, s := range sum.BySource {
		if s.Total() == 0 && s.Requests == 0 && s.UserTurns == 0 {
			continue
		}
		if s.Quality == event.QualityDegraded || s.Quality == event.QualityEstimated {
			return StatusPartial
		}
	}
	return StatusAvailable
}

func periodWindows(now time.Time, loc *time.Location, cal metric.Calendar) map[string]metric.Window {
	today, _ := metric.ParseWindow(true, "", "", "", now, loc)
	d7, _ := metric.ParseWindow(false, "7d", "", "", now, loc)
	d30, _ := metric.ParseWindow(false, "30d", "", "", now, loc)
	w53 := metric.Window{Label: "近 53 周"}
	if cal.WindowFrom != "" {
		t, err := time.ParseInLocation("2006-01-02", cal.WindowFrom, loc)
		if err == nil {
			w53.From = t
		}
	}
	w53.To = midnight(now, loc).AddDate(0, 0, 1)
	return map[string]metric.Window{
		PeriodAll:   {Label: "有账本以来"},
		PeriodToday: today,
		Period7d:    d7,
		Period30d:   d30,
		Period53w:   w53,
	}
}

func midnight(now time.Time, loc *time.Location) time.Time {
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func filterWindow(events []event.UsageEvent, turns []event.TurnEvent, w metric.Window, loc *time.Location) ([]event.UsageEvent, []event.TurnEvent) {
	if w.IsAll() {
		return events, turns
	}
	var evs []event.UsageEvent
	for _, e := range events {
		if w.Contains(e.Timestamp, loc) {
			evs = append(evs, e)
		}
	}
	var ts []event.TurnEvent
	for _, t := range turns {
		if w.Contains(t.Timestamp, loc) {
			ts = append(ts, t)
		}
	}
	return evs, ts
}

func projectPeriod(id string, w metric.Window, in Input, now time.Time, loc *time.Location, status string, cached metric.Summary) Period {
	unavailable := status == StatusUnavailable
	sum := cached
	if id != PeriodAll {
		evs, turns := filterWindow(in.Events, in.Turns, w, loc)
		sum = metric.AggregateAt(evs, turns, now, loc)
	}
	from := rangeFrom(w)
	to := now.Format("2006-01-02")
	if !w.To.IsZero() {
		to = w.To.AddDate(0, 0, -1).Format("2006-01-02")
	}
	p := Period{
		Range: DateRange{
			From:  from,
			To:    to,
			Label: w.Label,
		},
		Portrait: projectPortrait(sum, in.PortraitSeed, unavailable),
	}
	if unavailable {
		p.Totals = emptyTotals()
		p.HitRate = HitRate{Display: emDash}
		p.Requests = unavailableCount()
		p.UserTurns = unavailableCount()
		p.ActiveDays = unavailableCount()
		p.CurrentStreak = unavailableCount()
		p.LongestStreak = unavailableCount()
		p.Peak = Peak{Total: unavailableComponent(), Scope: id}
		p.ByAgent = []Breakdown{}
		p.ByVendor = []Breakdown{}
		if in.IncludeModels {
			p.ByModel = []Breakdown{}
		}
		return p
	}
	all := sum.All
	p.Totals = totalsFromSlice(all)
	p.HitRate = hitRateOf(all.Miss, all.CacheRead, all.CacheCreate, false)
	p.Requests = availableCount(all.Requests)
	p.UserTurns = availableCount(all.UserTurns)
	active := int64(0)
	for _, d := range sum.Calendar.All.Days {
		if d.Total > 0 {
			if from != nil && d.Date < *from {
				continue
			}
			if d.Date > to {
				continue
			}
			active++
		}
	}
	p.ActiveDays = availableCount(active)
	p.CurrentStreak = availableCount(int64(sum.Calendar.All.Stats.CurrentStreak))
	p.LongestStreak = availableCount(int64(sum.Calendar.All.Stats.LongestStreak))
	p.Peak = projectPeak(sum.Calendar, id)
	p.ByAgent = projectBreakdown(sum.BySource, all.Total(), true, false)
	p.ByVendor = projectBreakdown(sum.ByVendor, all.Total(), false, true)
	if in.IncludeModels {
		p.ByModel = projectModels(sum.ByModel, all.Total())
	}
	if in.IncludeCost {
		c := costFrom(all)
		p.Cost = &c
	}
	return p
}

func rangeFrom(w metric.Window) *string {
	if w.From.IsZero() {
		return nil
	}
	s := w.From.Format("2006-01-02")
	return &s
}

func emptyTotals() Totals {
	u := unavailableComponent()
	return Totals{Miss: u, CacheRead: u, CacheCreate: u, Output: u, Total: u}
}

func totalsFromSlice(s metric.Slice) Totals {
	return Totals{
		Miss:        availableInt(s.Miss),
		CacheRead:   availableInt(s.CacheRead),
		CacheCreate: availableInt(s.CacheCreate),
		Output:      availableInt(s.Output),
		Total:       availableInt(s.Total()),
	}
}

func projectPeak(cal metric.Calendar, scope string) Peak {
	st := cal.All.Stats
	if st.PeakDate == "" || st.PeakTotal <= 0 {
		return Peak{Total: unavailableComponent(), Scope: scope}
	}
	return Peak{
		Date:  st.PeakDate,
		Total: availableInt(st.PeakTotal),
		Scope: scope,
	}
}

func projectPortrait(sum metric.Summary, seed string, unavailable bool) Portrait {
	if unavailable {
		return Portrait{State: profile.StateNone, Primary: emDash, Tags: []string{}}
	}
	p := profile.Evaluate(sum, seed)
	tags := p.Tags
	if tags == nil {
		tags = []string{}
	}
	return Portrait{State: p.State, Primary: p.Primary, Tags: tags}
}

func projectBreakdown(rows []metric.Slice, all int64, asSource, asVendor bool) []Breakdown {
	type acc struct {
		id, label string
		s         metric.Slice
	}
	merged := map[string]*acc{}
	order := []string{}
	for _, s := range rows {
		if s.Total() == 0 && s.Requests == 0 {
			continue
		}
		id, label := s.ID, s.Label
		if asSource {
			id, label = publicSource(s.ID)
		}
		if asVendor {
			id, label = publicVendor(s.ID)
		}
		if a, ok := merged[id]; ok {
			a.s.Miss = satAdd(a.s.Miss, s.Miss)
			a.s.CacheRead = satAdd(a.s.CacheRead, s.CacheRead)
			a.s.CacheCreate = satAdd(a.s.CacheCreate, s.CacheCreate)
			a.s.Output = satAdd(a.s.Output, s.Output)
			a.s.Requests = satAdd(a.s.Requests, s.Requests)
			a.s.UserTurns = satAdd(a.s.UserTurns, s.UserTurns)
			continue
		}
		cp := s
		cp.ID, cp.Label = id, label
		merged[id] = &acc{id: id, label: label, s: cp}
		order = append(order, id)
	}
	list := make([]acc, 0, len(order))
	for _, id := range order {
		list = append(list, *merged[id])
	}
	sort.SliceStable(list, func(i, j int) bool {
		ti, tj := list[i].s.Total(), list[j].s.Total()
		if ti != tj {
			return ti > tj
		}
		return list[i].id < list[j].id
	})
	var rest metric.Slice
	out := []Breakdown{}
	for i, a := range list {
		if i < maxBreakdown {
			out = append(out, breakdownRow(a.id, a.label, a.s, all))
			continue
		}
		rest.Miss = satAdd(rest.Miss, a.s.Miss)
		rest.CacheRead = satAdd(rest.CacheRead, a.s.CacheRead)
		rest.CacheCreate = satAdd(rest.CacheCreate, a.s.CacheCreate)
		rest.Output = satAdd(rest.Output, a.s.Output)
		rest.Requests = satAdd(rest.Requests, a.s.Requests)
	}
	if rest.Total() > 0 || rest.Requests > 0 {
		id, label := AgentOtherID, AgentOtherLabel
		if asVendor {
			id, label = VendorOtherID, VendorOtherLabel
		}
		out = append(out, breakdownRow(id, label, rest, all))
	}
	if out == nil {
		return []Breakdown{}
	}
	return out
}

func projectModels(rows []metric.ModelSlice, all int64) []Breakdown {
	var slices []metric.Slice
	for _, m := range rows {
		id, label := publicModel(m.Model, m.Vendor)
		s := m.Slice
		s.ID, s.Label = id, label
		slices = append(slices, s)
	}
	return projectBreakdown(slices, all, false, false)
}

func breakdownRow(id, label string, s metric.Slice, all int64) Breakdown {
	q := string(s.Quality)
	if q == "" {
		q = string(event.QualityAuthoritative)
	}
	return Breakdown{
		ID:       id,
		Label:    label,
		Totals:   totalsFromSlice(s),
		Share:    metric.FormatShare(s.Total(), all),
		HitRate:  hitRateOf(s.Miss, s.CacheRead, s.CacheCreate, false),
		Requests: availableCount(s.Requests),
		Quality:  q,
	}
}

func costFrom(s metric.Slice) Cost {
	v := metric.View(s)
	st := v.CostStatus
	if st == "" {
		st = price.StatusUnavailable
	}
	display := v.CostUSD
	if display == "" {
		display = emDash
	}
	return Cost{
		Status:         st,
		USDMicro:       s.CostMicro,
		Display:        display,
		PricedTokens:   s.PricedTokens,
		UnpricedTokens: s.UnpricedTokens,
		PriceCardID:    price.CardVersion,
		VerifiedAt:     price.CardVersion,
	}
}

func buildSeries(sum metric.Summary, dates []string, windowTo string, unavailable, includeModels bool) []Series {
	out := []Series{seriesFromDays("all", SeriesAllID, "All", sum.Calendar.All.Days, dates, windowTo, unavailable)}
	type keyed struct {
		id, label string
		days      []metric.Day
		total     int64
	}
	var agents []keyed
	for id, ser := range sum.Calendar.BySource {
		pid, label := publicSource(id)
		total := int64(0)
		for _, d := range ser.Days {
			if d.Date >= sum.Calendar.WindowFrom && d.Date <= windowTo {
				total = satAdd(total, d.Total)
			}
		}
		if total == 0 {
			continue
		}
		agents = append(agents, keyed{id: pid, label: label, days: ser.Days, total: total})
	}
	sort.SliceStable(agents, func(i, j int) bool {
		if agents[i].total != agents[j].total {
			return agents[i].total > agents[j].total
		}
		return agents[i].id < agents[j].id
	})
	for i, a := range agents {
		if i >= maxSeries {
			break
		}
		out = append(out, seriesFromDays("agent", a.id, a.label, a.days, dates, windowTo, unavailable))
	}
	var vendors []keyed
	for id, ser := range sum.Calendar.ByVendor {
		pid, label := publicVendor(id)
		total := int64(0)
		for _, d := range ser.Days {
			if d.Date >= sum.Calendar.WindowFrom && d.Date <= windowTo {
				total = satAdd(total, d.Total)
			}
		}
		if total == 0 {
			continue
		}
		vendors = append(vendors, keyed{id: pid, label: label, days: ser.Days, total: total})
	}
	sort.SliceStable(vendors, func(i, j int) bool {
		if vendors[i].total != vendors[j].total {
			return vendors[i].total > vendors[j].total
		}
		return vendors[i].id < vendors[j].id
	})
	for i, v := range vendors {
		if i >= maxSeries {
			break
		}
		out = append(out, seriesFromDays("vendor", v.id, v.label, v.days, dates, windowTo, unavailable))
	}
	_ = includeModels
	return out
}
