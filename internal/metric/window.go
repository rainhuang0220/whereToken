package metric

import (
	"fmt"
	"strconv"
	"strings"
	"time"
)

// Window is a local-timezone filter over event timestamps.
// From is inclusive, To is exclusive. Zero From/To means that side is open.
type Window struct {
	Today bool
	Days  int
	From  time.Time
	To    time.Time
	Label string
}

func (w Window) IsAll() bool {
	return !w.Today && w.Days == 0 && w.From.IsZero() && w.To.IsZero()
}

func (w Window) Bounded() bool {
	return !w.From.IsZero() && !w.To.IsZero()
}

func (w Window) Contains(ts time.Time, loc *time.Location) bool {
	if w.IsAll() {
		return true
	}
	if ts.IsZero() {
		return false
	}
	if loc == nil {
		loc = time.Local
	}
	t := ts.In(loc)
	if !w.From.IsZero() && t.Before(w.From) {
		return false
	}
	if !w.To.IsZero() && !t.Before(w.To) {
		return false
	}
	return true
}

func (w Window) Previous() Window {
	if w.From.IsZero() || w.To.IsZero() {
		return Window{}
	}
	dur := w.To.Sub(w.From)
	return Window{From: w.From.Add(-dur), To: w.From, Label: "previous"}
}

func ParseSince(s string) (int, error) {
	s = strings.ToLower(strings.TrimSpace(s))
	if s == "" {
		return 0, fmt.Errorf("invalid --since (use Nd such as 7d)")
	}
	if s == "today" {
		return 1, nil
	}
	if !strings.HasSuffix(s, "d") {
		return 0, fmt.Errorf("invalid --since %q (use Nd such as 7d)", s)
	}
	n, err := strconv.Atoi(strings.TrimSpace(strings.TrimSuffix(s, "d")))
	if err != nil || n <= 0 {
		return 0, fmt.Errorf("invalid --since %q (use Nd such as 7d)", s)
	}
	return n, nil
}

func ParseClock(s string, loc *time.Location, end bool) (time.Time, error) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, fmt.Errorf("empty time")
	}
	if loc == nil {
		loc = time.Local
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t.In(loc), nil
	}
	for _, layout := range []string{"2006-01-02T15:04:05", "2006-01-02T15:04"} {
		if t, err := time.ParseInLocation(layout, s, loc); err == nil {
			return t, nil
		}
	}
	t, err := time.ParseInLocation("2006-01-02", s, loc)
	if err != nil {
		return time.Time{}, fmt.Errorf("invalid date %q (use YYYY-MM-DD)", s)
	}
	if end {
		return t.AddDate(0, 0, 1), nil
	}
	return t, nil
}

func ParseWindow(today bool, since, from, to string, now time.Time, loc *time.Location) (Window, error) {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)
	since, from, to = strings.TrimSpace(since), strings.TrimSpace(from), strings.TrimSpace(to)
	n := 0
	if today {
		n++
	}
	if since != "" {
		n++
	}
	if from != "" || to != "" {
		n++
	}
	if n > 1 {
		return Window{}, fmt.Errorf("use only one of --today, --since, or --from/--to")
	}
	if today || strings.EqualFold(since, "today") {
		start := midnight(now, loc)
		return Window{
			Today: true,
			Days:  1,
			From:  start,
			To:    start.AddDate(0, 0, 1),
			Label: "今天 " + start.Format("2006-01-02"),
		}, nil
	}
	if since != "" {
		days, err := ParseSince(since)
		if err != nil {
			return Window{}, err
		}
		start := midnight(now, loc).AddDate(0, 0, -(days - 1))
		return Window{
			Days:  days,
			From:  start,
			To:    midnight(now, loc).AddDate(0, 0, 1),
			Label: fmt.Sprintf("近 %d 天", days),
		}, nil
	}
	if from == "" && to == "" {
		return Window{Label: "有账本以来"}, nil
	}
	w := Window{}
	if from != "" {
		t, err := ParseClock(from, loc, false)
		if err != nil {
			return Window{}, err
		}
		w.From = t
	}
	if to != "" {
		t, err := ParseClock(to, loc, true)
		if err != nil {
			return Window{}, err
		}
		w.To = t
	}
	if !w.From.IsZero() && !w.To.IsZero() && !w.From.Before(w.To) {
		return Window{}, fmt.Errorf("--from must be before --to")
	}
	w.Label = windowLabel(w)
	return w, nil
}

func ParseSinceQuery(q string, now time.Time, loc *time.Location) (Window, error) {
	q = strings.TrimSpace(q)
	switch q {
	case "", "all":
		return Window{Label: "有账本以来"}, nil
	case "today":
		return ParseWindow(true, "", "", "", now, loc)
	default:
		return ParseWindow(false, q, "", "", now, loc)
	}
}

func midnight(now time.Time, loc *time.Location) time.Time {
	t := now.In(loc)
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, loc)
}

func windowLabel(w Window) string {
	switch {
	case !w.From.IsZero() && !w.To.IsZero():
		return w.From.Format("2006-01-02") + " … " + w.To.Add(-time.Nanosecond).Format("2006-01-02")
	case !w.From.IsZero():
		return w.From.Format("2006-01-02") + " 起"
	case !w.To.IsZero():
		return "至 " + w.To.Add(-time.Nanosecond).Format("2006-01-02")
	default:
		return "有账本以来"
	}
}

type Compare struct {
	PreviousTotal int64          `json:"previous_total"`
	DeltaPct      *float64       `json:"delta_pct"`
	BySource      []CompareSlice `json:"by_source"`
}

type CompareSlice struct {
	ID        string   `json:"id"`
	Label     string   `json:"label"`
	Total     int64    `json:"total"`
	Previous  int64    `json:"previous"`
	DeltaPct  *float64 `json:"delta_pct"`
	DeltaText string   `json:"delta_text"`
}

func NewCompare(cur, prev Summary) Compare {
	out := Compare{PreviousTotal: prev.All.Total()}
	out.DeltaPct = pctChange(cur.All.Total(), prev.All.Total())
	prevBy := map[string]Slice{}
	for _, s := range prev.BySource {
		prevBy[s.ID] = s
	}
	seen := map[string]struct{}{}
	for _, s := range cur.BySource {
		seen[s.ID] = struct{}{}
		p := prevBy[s.ID]
		out.BySource = append(out.BySource, CompareSlice{
			ID:        s.ID,
			Label:     s.Label,
			Total:     s.Total(),
			Previous:  p.Total(),
			DeltaPct:  pctChange(s.Total(), p.Total()),
			DeltaText: formatDelta(s.Total(), p.Total()),
		})
	}
	for _, s := range prev.BySource {
		if _, ok := seen[s.ID]; ok {
			continue
		}
		out.BySource = append(out.BySource, CompareSlice{
			ID:        s.ID,
			Label:     s.Label,
			Total:     0,
			Previous:  s.Total(),
			DeltaPct:  pctChange(0, s.Total()),
			DeltaText: formatDelta(0, s.Total()),
		})
	}
	return out
}

func pctChange(cur, prev int64) *float64 {
	if prev == 0 {
		return nil
	}
	v := float64(cur-prev) / float64(prev) * 100
	return &v
}

func formatDelta(cur, prev int64) string {
	if prev == 0 {
		if cur == 0 {
			return "—"
		}
		return "new"
	}
	v := float64(cur-prev) / float64(prev) * 100
	sign := "+"
	if v < 0 {
		sign = ""
	}
	return fmt.Sprintf("%s%.0f%%", sign, v)
}
