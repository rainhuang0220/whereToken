package metric

import (
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

type Day struct {
	Date                                 string
	Miss, CacheRead, CacheCreate, Output int64
	Total                                int64
	Level                                int
}

type CalendarStats struct {
	PeakDate      string
	PeakTotal     int64
	CurrentStreak int
	LongestStreak int
}

type CalendarSeries struct {
	Days  []Day
	Stats CalendarStats
}

type Calendar struct {
	WeekStart  string
	Timezone   string
	WindowFrom string
	WindowTo   string
	All        CalendarSeries
	BySource   map[string]CalendarSeries
	ByVendor   map[string]CalendarSeries
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
	return Calendar{
		WeekStart:  "monday",
		Timezone:   loc.String(),
		WindowFrom: from.Format("2006-01-02"),
		WindowTo:   today.Format("2006-01-02"),
		All:        finishSeries(bucketDays(events, loc), today),
		BySource:   map[string]CalendarSeries{},
		ByVendor:   map[string]CalendarSeries{},
	}
}

func finishSeries(days []Day, today time.Time) CalendarSeries {
	return CalendarSeries{Days: days, Stats: computeStats(days, today)}
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

func bucketDays(events []event.UsageEvent, loc *time.Location) []Day {
	index := map[string]int{}
	var days []Day
	for _, e := range events {
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
