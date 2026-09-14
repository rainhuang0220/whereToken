package publicprofile

import (
	"math"
	"sort"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
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

func seriesFromCounts(dim, id, label, metricName string, counts map[string]int64, dates []string, windowTo string, unavailable bool) Series {
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
		if v := counts[date]; v > 0 {
			states[i] = CellActive
			values[i] = v
			continue
		}
		states[i] = CellEmpty
	}
	return Series{
		Dimension: dim,
		ID:        id,
		Label:     label,
		Metric:    metricName,
		Values:    values,
		Levels:    levelsFromVisible(values, states),
		States:    states,
	}
}

func dayTotals(days []metric.Day) map[string]int64 {
	out := make(map[string]int64, len(days))
	for _, d := range days {
		if d.Total > 0 {
			out[d.Date] = d.Total
		}
	}
	return out
}

func requestCounts(events []event.UsageEvent, loc *time.Location, keep func(event.UsageEvent) bool) map[string]int64 {
	out := map[string]int64{}
	if loc == nil {
		loc = time.UTC
	}
	for _, e := range events {
		if e.SkipRequest || e.Timestamp.IsZero() {
			continue
		}
		if keep != nil && !keep(e) {
			continue
		}
		date := e.Timestamp.In(loc).Format("2006-01-02")
		out[date] = satAdd(out[date], 1)
	}
	return out
}

// levelsFromVisible uses only the 53-week cells being rendered.
// History outside the wall must not change intensity.
func levelsFromVisible(values []int64, states []string) []int {
	var nonzero []int64
	for i, v := range values {
		if i < len(states) && states[i] == CellActive && v > 0 {
			nonzero = append(nonzero, v)
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
	q1 := quantileNearest(nonzero, 0.20)
	q2 := quantileNearest(nonzero, 0.40)
	q3 := quantileNearest(nonzero, 0.60)
	q4 := quantileNearest(nonzero, 0.80)
	if q1 == nonzero[n-1] {
		return 3
	}
	switch {
	case total <= q1:
		return 1
	case total <= q2:
		return 2
	case total <= q3:
		return 3
	case total <= q4:
		return 4
	default:
		return 5
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
