package metric

import (
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

type Day struct {
	Date          string `json:"date"`
	Miss          int64  `json:"miss"`
	CacheRead     int64  `json:"cache_read"`
	CacheCreate   int64  `json:"cache_create"`
	Output        int64  `json:"output"`
	Total         int64  `json:"total"`
	MissM         string `json:"miss_m"`
	CacheReadM    string `json:"cache_read_m"`
	CacheCreateM  string `json:"cache_create_m"`
	OutputM       string `json:"output_m"`
	TotalM        string `json:"total_m"`
	Level         int    `json:"level"`
}

type CalendarStats struct {
	PeakDate      string `json:"peak_date"`
	PeakTotal     int64  `json:"peak_total"`
	PeakTotalM    string `json:"peak_total_m"`
	CurrentStreak int    `json:"current_streak"`
	LongestStreak int    `json:"longest_streak"`
}

type CalendarSeries struct {
	Days  []Day         `json:"days"`
	Stats CalendarStats `json:"stats"`
}

type Calendar struct {
	WeekStart  string                     `json:"week_start"`
	Timezone   string                     `json:"timezone"`
	WindowFrom string                     `json:"window_from"`
	WindowTo   string                     `json:"window_to"`
	All        CalendarSeries             `json:"all"`
	BySource   map[string]CalendarSeries  `json:"by_source"`
	ByVendor   map[string]CalendarSeries  `json:"by_vendor"`
}

func BuildCalendar(events []event.UsageEvent, loc *time.Location, now time.Time) Calendar {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	today := truncateDay(now)
	weekday := int(today.Weekday())
	if weekday == 0 {
		weekday = 7
	}
	weekStart := today.AddDate(0, 0, -(weekday - 1))
	from := weekStart.AddDate(0, 0, -52*7)
	cal := Calendar{
		WeekStart:  "monday",
		Timezone:   loc.String(),
		WindowFrom: from.Format("2006-01-02"),
		WindowTo:   today.Format("2006-01-02"),
		All:        finishSeries(bucketDays(events, loc, nil), today),
		BySource:   map[string]CalendarSeries{},
		ByVendor:   map[string]CalendarSeries{},
	}
	sources := map[string]struct{}{}
	vendors := map[string]struct{}{}
	for _, e := range events {
		sources[e.Source] = struct{}{}
		vendors[e.Vendor] = struct{}{}
	}
	for id := range sources {
		src := id
		cal.BySource[id] = finishSeries(bucketDays(events, loc, func(e event.UsageEvent) bool { return e.Source == src }), today)
	}
	for id := range vendors {
		vend := id
		cal.ByVendor[id] = finishSeries(bucketDays(events, loc, func(e event.UsageEvent) bool { return e.Vendor == vend }), today)
	}
	return cal
}

func finishSeries(days []Day, today time.Time) CalendarSeries {
	if days == nil {
		days = []Day{}
	}
	days = assignLevels(days)
	for i := range days {
		days[i].MissM = FormatM(days[i].Miss)
		days[i].CacheReadM = FormatM(days[i].CacheRead)
		days[i].CacheCreateM = FormatM(days[i].CacheCreate)
		days[i].OutputM = FormatM(days[i].Output)
		days[i].TotalM = FormatM(days[i].Total)
	}
	stats := computeStats(days, today)
	stats.PeakTotalM = FormatM(stats.PeakTotal)
	return CalendarSeries{Days: days, Stats: stats}
}

func assignLevels(days []Day) []Day {
	var nonzero []int64
	for _, d := range days {
		if d.Total > 0 {
			nonzero = append(nonzero, d.Total)
		}
	}
	sort.Slice(nonzero, func(i, j int) bool { return nonzero[i] < nonzero[j] })
	for i := range days {
		days[i].Level = intensityLevel(days[i].Total, nonzero)
	}
	return days
}

func intensityLevel(total int64, nonzero []int64) int {
	if total <= 0 || len(nonzero) == 0 {
		return 0
	}
	n := len(nonzero)
	q1 := quantileNearest(nonzero, 0.25)
	q2 := quantileNearest(nonzero, 0.50)
	q3 := quantileNearest(nonzero, 0.75)
	if q1 == nonzero[n-1] {
		return 2
	}
	switch {
	case total <= q1:
		return 1
	case total <= q2:
		return 2
	case total <= q3:
		return 3
	default:
		return 4
	}
}

func quantileNearest(sorted []int64, p float64) int64 {
	n := len(sorted)
	if n == 0 {
		return 0
	}
	idx := int(ceilFloat(float64(n)*p)) - 1
	if idx < 0 {
		idx = 0
	}
	if idx >= n {
		idx = n - 1
	}
	return sorted[idx]
}

func ceilFloat(x float64) int {
	i := int(x)
	if float64(i) < x {
		return i + 1
	}
	return i
}

func computeStats(days []Day, today time.Time) CalendarStats {
	stats := CalendarStats{}
	used := map[string]int64{}
	for _, d := range days {
		used[d.Date] = d.Total
		if d.Total > stats.PeakTotal || (d.Total == stats.PeakTotal && d.Date > stats.PeakDate) {
			stats.PeakTotal = d.Total
			stats.PeakDate = d.Date
		}
	}
	if stats.PeakTotal == 0 {
		stats.PeakDate = ""
	}
	stats.CurrentStreak = currentStreak(used, today)
	stats.LongestStreak = longestStreak(used, today)
	return stats
}

func currentStreak(used map[string]int64, today time.Time) int {
	anchor := today
	if used[today.Format("2006-01-02")] == 0 {
		anchor = today.AddDate(0, 0, -1)
	}
	n := 0
	for {
		if used[anchor.Format("2006-01-02")] == 0 {
			break
		}
		n++
		anchor = anchor.AddDate(0, 0, -1)
	}
	return n
}

func longestStreak(used map[string]int64, today time.Time) int {
	if len(used) == 0 {
		return 0
	}
	first := ""
	for d := range used {
		if first == "" || d < first {
			first = d
		}
	}
	start, err := time.ParseInLocation("2006-01-02", first, today.Location())
	if err != nil {
		return 0
	}
	best, run := 0, 0
	for d := start; !d.After(today); d = d.AddDate(0, 0, 1) {
		if used[d.Format("2006-01-02")] > 0 {
			run++
			if run > best {
				best = run
			}
			continue
		}
		run = 0
	}
	return best
}

func truncateDay(t time.Time) time.Time {
	y, m, d := t.Date()
	return time.Date(y, m, d, 0, 0, 0, 0, t.Location())
}

func bucketDays(events []event.UsageEvent, loc *time.Location, keep func(event.UsageEvent) bool) []Day {
	index := map[string]int{}
	var days []Day
	for _, e := range events {
		if keep != nil && !keep(e) {
			continue
		}
		if e.Timestamp.IsZero() {
			continue
		}
		date := e.Timestamp.In(loc).Format("2006-01-02")
		total := e.Miss + e.CacheRead + e.CacheCreate + e.Output
		if i, ok := index[date]; ok {
			days[i].Miss += e.Miss
			days[i].CacheRead += e.CacheRead
			days[i].CacheCreate += e.CacheCreate
			days[i].Output += e.Output
			days[i].Total += total
			continue
		}
		index[date] = len(days)
		days = append(days, Day{
			Date:        date,
			Miss:        e.Miss,
			CacheRead:   e.CacheRead,
			CacheCreate: e.CacheCreate,
			Output:      e.Output,
			Total:       total,
		})
	}
	sort.Slice(days, func(i, j int) bool { return days[i].Date < days[j].Date })
	var out []Day
	for _, d := range days {
		if d.Total > 0 {
			out = append(out, d)
		}
	}
	return out
}
