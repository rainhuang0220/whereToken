package publicprofile

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
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

const (
	ProvenanceLocal         = "local_sanitized_snapshot"
	ProvenanceSyntheticDemo = "synthetic_demo"
	RefreshManualPublish    = "manual_publish"
	RefreshCommittedFixture = "committed_fixture"
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

	canonicalEvents := metric.CanonicalEvents(in.Events)
	in.Events = canonicalEvents
	allSum := metric.AggregateAt(canonicalEvents, in.Turns, now, loc)
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
			Kind:        ProvenanceLocal,
			RefreshMode: RefreshManualPublish,
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
		Notices: notices(status, in.IncludeCost, allSum, in),
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

	dates := wallDates(allSum.Calendar.WindowFrom)
	snap.Activity = Activity{
		WeekStart: "monday",
		From:      allSum.Calendar.WindowFrom,
		To:        allSum.Calendar.WindowTo,
		Dates:     dates,
		Series:    buildSeries(allSum, dates, allSum.Calendar.WindowTo, unavailable, in),
	}
	refreshSnapshotID(&snap)
	return snap, nil
}

// MarkSyntheticDemo gives committed documentation fixtures an explicit,
// machine-checkable provenance that production publication rejects.
func MarkSyntheticDemo(s *Snapshot) {
	if s == nil {
		return
	}
	s.Provenance = Provenance{Kind: ProvenanceSyntheticDemo, RefreshMode: RefreshCommittedFixture, LiveSync: false}
	refreshSnapshotID(s)
}

func refreshSnapshotID(s *Snapshot) {
	if s == nil {
		return
	}
	clone := *s
	clone.SnapshotID = ""
	clone.GeneratedAt = ""
	raw, err := json.Marshal(clone)
	if err != nil {
		s.SnapshotID = ""
		return
	}
	sum := sha256.Sum256(raw)
	s.SnapshotID = "sha256:" + hex.EncodeToString(sum[:])
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

func notices(status string, includeCost bool, sum metric.Summary, in Input) []string {
	var out []string
	if status == StatusPartial {
		out = append(out, NoticePartialUsage)
	}
	if includeCost && sum.All.CostStatus == price.StatusPartial {
		out = append(out, NoticeUnpricedCost)
	}
	for _, s := range sum.BySource {
		id, _ := publicSource(s.ID)
		if tokenReason(id, in) == ReasonAccountAPISkipped && tokensUnavailable(s) && s.Requests > 0 {
			out = append(out, NoticeAccountAPISkipped)
			break
		}
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
	periodEvents := in.Events
	if id != PeriodAll {
		evs, turns := filterWindow(in.Events, in.Turns, w, loc)
		periodEvents = evs
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
		if in.IncludeCost {
			c := unavailableCost()
			p.Cost = &c
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
	p.ByAgent = projectBreakdown(sum.BySource, all.Total(), true, false, in, loc)
	p.ByVendor = projectBreakdown(sum.ByVendor, all.Total(), false, true, in, loc)
	if in.IncludeModels {
		p.ByModel = projectModels(sum.ByModel, all.Total(), in, loc)
	}
	if in.IncludeCost {
		c := costFrom(all, periodEvents)
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
	st := tokenStatusOf(s)
	if st == StatusUnavailable {
		return emptyTotals()
	}
	return Totals{
		Miss:        intComponent(s.Miss, st),
		CacheRead:   intComponent(s.CacheRead, st),
		CacheCreate: intComponent(s.CacheCreate, st),
		Output:      intComponent(s.Output, st),
		Total:       intComponent(s.Total(), st),
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

func projectBreakdown(rows []metric.Slice, all int64, asSource, asVendor bool, in Input, loc *time.Location) []Breakdown {
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
		if i < maxBreakdown || (asSource && cloudSource(a.id)) {
			out = append(out, breakdownRow(a.id, a.label, a.s, all, asSource, in, loc))
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
		out = append(out, breakdownRow(id, label, rest, all, asSource, in, loc))
	}
	if out == nil {
		return []Breakdown{}
	}
	return out
}

func projectModels(rows []metric.ModelSlice, all int64, in Input, loc *time.Location) []Breakdown {
	var slices []metric.Slice
	for _, m := range rows {
		id, label := publicModel(m.Model, m.Vendor)
		s := m.Slice
		s.ID, s.Label = id, label
		slices = append(slices, s)
	}
	return projectBreakdown(slices, all, false, false, in, loc)
}

func breakdownRow(id, label string, s metric.Slice, all int64, asSource bool, in Input, loc *time.Location) Breakdown {
	q := string(s.Quality)
	if q == "" {
		q = string(event.QualityAuthoritative)
	}
	unavailable := tokensUnavailable(s)
	share := emDash
	if !unavailable {
		if s.Total() == 0 {
			share = "0.0%"
		} else {
			share = metric.FormatShare(s.Total(), all)
		}
	}
	cov := coverageFromSlice(id, s, asSource, in, loc)
	return Breakdown{
		ID:       id,
		Label:    label,
		Totals:   totalsFromSlice(s),
		Share:    share,
		HitRate:  hitRateOf(s.Miss, s.CacheRead, s.CacheCreate, unavailable),
		Requests: availableCount(s.Requests),
		Quality:  q,
		Coverage: cov,
	}
}

func costFrom(s metric.Slice, events []event.UsageEvent) Cost {
	v := metric.View(s)
	st := v.CostStatus
	if st == "" {
		st = price.StatusUnavailable
	}
	display := v.CostUSD
	if display == "" {
		display = emDash
	}
	usd, priced, unpriced := s.CostMicro, s.PricedTokens, s.UnpricedTokens
	return Cost{
		Status:         st,
		USDMicro:       &usd,
		Display:        display,
		PricedTokens:   &priced,
		UnpricedTokens: &unpriced,
		PriceCardID:    price.CardVersion,
		VerifiedAt:     costVerifiedAt(events),
	}
}

func unavailableCost() Cost {
	return Cost{Status: price.StatusUnavailable, Display: emDash, PriceCardID: price.CardVersion}
}

func costVerifiedAt(events []event.UsageEvent) *string {
	latest := ""
	for _, e := range events {
		rate, _, ok := price.Resolve(e.Vendor, e.Model, e.Timestamp)
		if !ok || !price.Event(e).OK {
			continue
		}
		meta, ok := price.SourceFor(rate.Source)
		if ok && meta.Verified > latest {
			latest = meta.Verified
		}
	}
	if latest == "" {
		return nil
	}
	return &latest
}

type seriesAcc struct {
	id, label string
	counts    map[string]int64
	total     int64
}

func buildSeries(sum metric.Summary, dates []string, windowTo string, unavailable bool, in Input) []Series {
	loc := in.Loc
	from, to := sum.Calendar.WindowFrom, windowTo
	out := []Series{seriesFromCounts("all", SeriesAllID, "All", MetricTokens, dayTotals(sum.Calendar.All.Days), dates, to, unavailable)}
	reqAll := requestCounts(in.Events, loc, nil)
	out = append(out, seriesFromCounts("all", SeriesAllID, "All", MetricRequests, reqAll, dates, to, unavailable))

	quality := map[string]metric.Slice{}
	for _, s := range sum.BySource {
		pid, _ := publicSource(s.ID)
		quality[pid] = s
	}

	tokenAgents := map[string]*seriesAcc{}
	for id, ser := range sum.Calendar.BySource {
		pid, label := publicSource(id)
		if tokensUnavailable(quality[pid]) {
			continue
		}
		counts := dayTotals(ser.Days)
		total := windowSum(counts, from, to)
		if total == 0 {
			continue
		}
		if a, ok := tokenAgents[pid]; ok {
			for date, v := range counts {
				a.counts[date] = satAdd(a.counts[date], v)
			}
			a.total = satAdd(a.total, total)
			continue
		}
		tokenAgents[pid] = &seriesAcc{id: pid, label: label, counts: counts, total: total}
	}

	requestAgents := map[string]*seriesAcc{}
	for _, e := range in.Events {
		if e.SkipRequest || e.Timestamp.IsZero() {
			continue
		}
		pid, label := publicSource(e.Source)
		if loc == nil {
			loc = time.UTC
		}
		date := e.Timestamp.In(loc).Format("2006-01-02")
		a := requestAgents[pid]
		if a == nil {
			a = &seriesAcc{id: pid, label: label, counts: map[string]int64{}}
			requestAgents[pid] = a
		}
		a.counts[date] = satAdd(a.counts[date], 1)
		if date >= from && date <= to {
			a.total = satAdd(a.total, 1)
		}
	}

	out = appendSeries(out, "agent", MetricTokens, mapValues(tokenAgents), dates, to, unavailable)
	out = appendSeries(out, "agent", MetricRequests, mapValues(requestAgents), dates, to, unavailable)

	tokenVendors := map[string]*seriesAcc{}
	for id, ser := range sum.Calendar.ByVendor {
		pid, label := publicVendor(id)
		counts := dayTotals(ser.Days)
		total := windowSum(counts, from, to)
		if total == 0 {
			continue
		}
		if a, ok := tokenVendors[pid]; ok {
			for date, v := range counts {
				a.counts[date] = satAdd(a.counts[date], v)
			}
			a.total = satAdd(a.total, total)
			continue
		}
		tokenVendors[pid] = &seriesAcc{id: pid, label: label, counts: counts, total: total}
	}
	requestVendors := map[string]*seriesAcc{}
	for _, e := range in.Events {
		if e.SkipRequest || e.Timestamp.IsZero() {
			continue
		}
		pid, label := publicVendor(e.Vendor)
		if loc == nil {
			loc = time.UTC
		}
		date := e.Timestamp.In(loc).Format("2006-01-02")
		a := requestVendors[pid]
		if a == nil {
			a = &seriesAcc{id: pid, label: label, counts: map[string]int64{}}
			requestVendors[pid] = a
		}
		a.counts[date] = satAdd(a.counts[date], 1)
		if date >= from && date <= to {
			a.total = satAdd(a.total, 1)
		}
	}
	out = appendSeries(out, "vendor", MetricTokens, mapValues(tokenVendors), dates, to, unavailable)
	out = appendSeries(out, "vendor", MetricRequests, mapValues(requestVendors), dates, to, unavailable)
	return out
}

func mapValues(in map[string]*seriesAcc) []seriesAcc {
	out := make([]seriesAcc, 0, len(in))
	for _, v := range in {
		if v.total == 0 {
			continue
		}
		out = append(out, *v)
	}
	sort.SliceStable(out, func(i, j int) bool {
		if out[i].total != out[j].total {
			return out[i].total > out[j].total
		}
		return out[i].id < out[j].id
	})
	return out
}

func appendSeries(out []Series, dim, metricName string, rows []seriesAcc, dates []string, windowTo string, unavailable bool) []Series {
	for i, a := range rows {
		if i >= maxSeries {
			break
		}
		out = append(out, seriesFromCounts(dim, a.id, a.label, metricName, a.counts, dates, windowTo, unavailable))
	}
	return out
}

func windowSum(counts map[string]int64, from, to string) int64 {
	var n int64
	for date, v := range counts {
		if date >= from && date <= to {
			n = satAdd(n, v)
		}
	}
	return n
}
