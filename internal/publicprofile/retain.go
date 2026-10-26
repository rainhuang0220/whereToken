package publicprofile

import (
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

// ObservationDay is one UTC day of a single account-API observation.
// The four components belong to that observation. They are not combined
// with another version of the same day.
type ObservationDay struct {
	Day                                  string
	Miss, CacheRead, CacheCreate, Output int64
}

// AccountFetchFailed reports that source produced an error and no fresh
// account-API events. A retained observation may then be reused. Local rows
// are not treated as the account measurement.
func AccountFetchFailed(events []event.UsageEvent, errs []string, source string) bool {
	for _, e := range events {
		if e.Source == source && e.Derivation == event.DeriveProviderAPI {
			return false
		}
	}
	return sourceError(source, errs) != ""
}

// EventsFromDays builds the retained rows stored for one observation.
// Each day is noon UTC so the calendar day does not move across the zones
// the profile renders. SkipRequest keeps the account total out of the
// request count.
func EventsFromDays(source string, days []ObservationDay) []event.UsageEvent {
	out := make([]event.UsageEvent, 0, len(days))
	for _, d := range days {
		if d.Miss+d.CacheRead+d.CacheCreate+d.Output == 0 || d.Day == "" {
			continue
		}
		ts, err := time.Parse("2006-01-02", d.Day)
		if err != nil {
			continue
		}
		out = append(out, event.UsageEvent{
			Source:      source,
			RequestID:   "retained:" + d.Day,
			Timestamp:   ts.Add(12 * time.Hour),
			Miss:        d.Miss,
			CacheRead:   d.CacheRead,
			CacheCreate: d.CacheCreate,
			Output:      d.Output,
			Quality:     event.QualityDegraded,
			Derivation:  event.DeriveRetained,
			SkipRequest: true,
		})
	}
	return out
}

// ApplyRetainedEvents strips local token counts for sources that have a
// retained observation, then appends that observation. The local request
// rows stay. Token totals are the observation alone.
func ApplyRetainedEvents(events, retained []event.UsageEvent) []event.UsageEvent {
	if len(retained) == 0 {
		return events
	}
	sources := map[string]struct{}{}
	for _, e := range retained {
		if e.Source != "" && e.Derivation == event.DeriveRetained {
			sources[e.Source] = struct{}{}
		}
	}
	if len(sources) == 0 {
		return events
	}
	out := make([]event.UsageEvent, 0, len(events)+len(retained))
	for _, e := range events {
		if _, ok := sources[e.Source]; ok && e.Derivation != event.DeriveRetained {
			e.Miss, e.CacheRead, e.CacheCreate, e.Output = 0, 0, 0, 0
			e.Quality = ""
			e.Derivation = ""
		}
		out = append(out, e)
	}
	return append(out, retained...)
}

// DaysFromSnapshot reads one measured agent's daily totals from a verified
// snapshot and splits that same snapshot's component vector across those
// days. A gap in the date list, a component sum that is not the total, or
// an unmeasured agent returns false. Missing days are not filled with zero.
func DaysFromSnapshot(snap Snapshot, source string) ([]ObservationDay, bool) {
	var row *Breakdown
	for i := range snap.Periods.All.ByAgent {
		if snap.Periods.All.ByAgent[i].ID == source {
			row = &snap.Periods.All.ByAgent[i]
			break
		}
	}
	if row == nil || !measuredAgent(*row) {
		return nil, false
	}
	parts, ok := componentVector(row.Totals)
	if !ok {
		return nil, false
	}
	var values []int64
	var dates []string
	for _, ser := range snap.Activity.Series {
		if ser.ID == source && (ser.Metric == "" || ser.Metric == MetricTokens) {
			values = ser.Values
			dates = snap.Activity.Dates
			break
		}
	}
	if len(dates) == 0 || len(dates) != len(values) || !contiguousDays(dates) {
		return nil, false
	}
	var sum int64
	var weights []int64
	var days []string
	for i, v := range values {
		if v < 0 {
			return nil, false
		}
		sum += v
		if v == 0 {
			continue
		}
		weights = append(weights, v)
		days = append(days, dates[i])
	}
	if sum != parts[4] || len(days) == 0 {
		return nil, false
	}
	alloc, ok := allocateComponents(weights, [4]int64{parts[0], parts[1], parts[2], parts[3]})
	if !ok {
		return nil, false
	}
	out := make([]ObservationDay, len(days))
	for i := range days {
		out[i] = ObservationDay{
			Day: days[i], Miss: alloc[i][0], CacheRead: alloc[i][1], CacheCreate: alloc[i][2], Output: alloc[i][3],
		}
	}
	return out, true
}

func componentVector(t Totals) ([5]int64, bool) {
	vals := []*int64{t.Miss.Value, t.CacheRead.Value, t.CacheCreate.Value, t.Output.Value, t.Total.Value}
	var out [5]int64
	for i, v := range vals {
		if v == nil || *v < 0 {
			return out, false
		}
		out[i] = *v
	}
	if out[0]+out[1]+out[2]+out[3] != out[4] {
		return out, false
	}
	return out, true
}

func contiguousDays(dates []string) bool {
	var prev time.Time
	for i, d := range dates {
		ts, err := time.Parse("2006-01-02", d)
		if err != nil {
			return false
		}
		if i > 0 && !ts.Equal(prev.AddDate(0, 0, 1)) {
			return false
		}
		prev = ts
	}
	return true
}

// allocateComponents splits one observation's component totals across day
// weights. The last day receives the remaining pool, so the components stay
// the original vector. It returns false when a day cannot be filled from
// that pool.
func allocateComponents(weights []int64, comps [4]int64) ([][4]int64, bool) {
	if len(weights) == 0 {
		return nil, false
	}
	out := make([][4]int64, len(weights))
	remC := comps
	var remW int64
	for _, w := range weights {
		if w <= 0 {
			return nil, false
		}
		remW += w
	}
	if remW != remC[0]+remC[1]+remC[2]+remC[3] {
		return nil, false
	}
	for i, w := range weights {
		if i == len(weights)-1 {
			if remC[0]+remC[1]+remC[2]+remC[3] != w {
				return nil, false
			}
			out[i] = remC
			return out, true
		}
		var assigned [4]int64
		var used int64
		type frac struct {
			idx int
			rem int64
		}
		order := make([]frac, 4)
		for c := 0; c < 4; c++ {
			assigned[c] = remC[c] * w / remW
			used += assigned[c]
			order[c] = frac{c, remC[c] * w % remW}
		}
		left := w - used
		for a := 0; a < 4 && left > 0; a++ {
			best := a
			for b := a + 1; b < 4; b++ {
				if order[b].rem > order[best].rem || (order[b].rem == order[best].rem && order[b].idx < order[best].idx) {
					best = b
				}
			}
			order[a], order[best] = order[best], order[a]
			c := order[a].idx
			if assigned[c] >= remC[c] {
				continue
			}
			assigned[c]++
			left--
		}
		if left != 0 {
			return nil, false
		}
		for c := 0; c < 4; c++ {
			remC[c] -= assigned[c]
			if remC[c] < 0 {
				return nil, false
			}
		}
		remW -= w
		out[i] = assigned
	}
	return nil, false
}
