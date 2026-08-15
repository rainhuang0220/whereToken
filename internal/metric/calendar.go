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
		All:        CalendarSeries{Days: bucketDays(events, loc)},
		BySource:   map[string]CalendarSeries{},
		ByVendor:   map[string]CalendarSeries{},
	}
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
