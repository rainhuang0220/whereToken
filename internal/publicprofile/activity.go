package publicprofile

import (
	"math"
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/metric"
)

func wallDates(from string) []string {
	start, err := time.Parse("2006-01-02", from)
	if err != nil {
		return nil
	}
	out := make([]string, 0, WallDays)
	for i := 0; i < WallDays; i++ {
		out = append(out, start.AddDate(0, 0, i).Format("2006-01-02"))
	}
	return out
}

func seriesFromDays(dim, id, label string, days []metric.Day, dates []string, windowTo string, unavailable bool) Series {
	byDate := make(map[string]metric.Day, len(days))
	for _, d := range days {
		byDate[d.Date] = d
	}
	values := make([]int64, len(dates))
	states := make([]string, len(dates))
	for i, date := range dates {
		if windowTo != "" && date > windowTo {
			states[i] = CellFuture
			continue
		}
		if unavailable {
			states[i] = CellUnknown
			continue
		}
		if d, ok := byDate[date]; ok && d.Total > 0 {
			states[i] = CellActive
			values[i] = d.Total
			continue
		}
		states[i] = CellEmpty
	}
	return Series{
		Dimension: dim,
		ID:        id,
		Label:     label,
		Values:    values,
		Levels:    levelsFromHistory(values, states, days),
		States:    states,
	}
}

// levelsFromHistory mirrors metric.Calendar intensity: thresholds come from
// the complete canonical series, while the returned cells remain 53-week.
func levelsFromHistory(values []int64, states []string, days []metric.Day) []int {
	var nonzero []int64
	for _, day := range days {
		if day.Total > 0 {
			nonzero = append(nonzero, day.Total)
		}
	}
	sort.Slice(nonzero, func(i, j int) bool { return nonzero[i] < nonzero[j] })
	out := make([]int, len(values))
	for i, v := range values {
		if i < len(states) && states[i] != CellActive {
			continue
		}
		out[i] = intensityLevel(v, nonzero)
	}
	return out
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
	return int(math.Ceil(x))
}
