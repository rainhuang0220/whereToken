// Package card builds a privacy-allowlisted public usage card and renders
// it as a deterministic static SVG. The renderer accepts only PublicCard —
// never scan.Result, metric.Summary, report.Snapshot, or raw events.
package card

import (
	"math"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/price"
)

const (
	SchemaVersion = 1
	WallWeeks     = 53
	WallDays      = WallWeeks * 7 // 371
)

const (
	DataAvailable   = "available"
	DataPartial     = "partial"
	DataUnavailable = "unavailable"
)

const (
	CellUnknown = "unknown"
	CellEmpty   = "empty"
	CellActive  = "active"
	CellFuture  = "future"
)

const (
	agentRestID    = "rest"
	agentRestLabel = "Rest"
	emDash         = "—"
)

// PublicCard is the explicit allowlist the SVG renderer may see.
type PublicCard struct {
	SchemaVersion     int
	Version           string
	DataStatus        string
	AsOfDate          string
	Range             DateRange
	AllTimeTokens     Quantity
	Last53WeeksTokens Quantity
	CurrentStreakDays int
	LongestStreakDays int
	ActiveDays53Weeks int
	Peak              Peak
	Cost              Cost
	Cells             []Cell
	Agents            []Agent
}

type DateRange struct {
	WeekStart  string
	WindowFrom string
	WindowTo   string
}

type Quantity struct {
	Raw     int64
	Display string
}

type Peak struct {
	Available     bool
	Date          string
	TokensRaw     int64
	TokensDisplay string
	VisibleInWall bool
}

type Cost struct {
	Status  string
	Display string
}

type Cell struct {
	Date          string
	State         string
	Level         int
	TokensRaw     int64
	TokensDisplay string
}

type Agent struct {
	ID            string
	Label         string
	TokensRaw     int64
	TokensDisplay string
	ShareText     string
	ShareRatio    float64
	Rest          bool
}

// StatusOf projects a safe availability flag from the canonical summary.
// Detected-but-empty tools are not partial; contributing degraded quality is.
func StatusOf(sum metric.Summary) string {
	if sum.All.Total() == 0 && sum.All.Requests == 0 && sum.All.UserTurns == 0 {
		return DataUnavailable
	}
	if sum.All.Quality == event.QualityDegraded || sum.All.Quality == event.QualityEstimated {
		return DataPartial
	}
	for _, s := range sum.BySource {
		if s.Total() == 0 && s.Requests == 0 && s.UserTurns == 0 {
			continue
		}
		if s.Quality == event.QualityDegraded || s.Quality == event.QualityEstimated {
			return DataPartial
		}
	}
	return DataAvailable
}

// NewView builds a PublicCard from a canonical metric.Summary and a
// precomputed availability flag. It copies BySource before sorting.
func NewView(sum metric.Summary, dataStatus, version string) PublicCard {
	cal := sum.Calendar
	unavailable := dataStatus == DataUnavailable
	allRaw := sum.All.Total()
	weekRaw, active := projectWindow(cal.All.Days, cal.WindowFrom, cal.WindowTo)
	card := PublicCard{
		SchemaVersion: SchemaVersion,
		Version:       version,
		DataStatus:    dataStatus,
		AsOfDate:      cal.WindowTo,
		Range: DateRange{
			WeekStart:  cal.WeekStart,
			WindowFrom: cal.WindowFrom,
			WindowTo:   cal.WindowTo,
		},
		AllTimeTokens:     quantity(allRaw, unavailable),
		Last53WeeksTokens: quantity(weekRaw, unavailable),
		CurrentStreakDays: cal.All.Stats.CurrentStreak,
		LongestStreakDays: cal.All.Stats.LongestStreak,
		ActiveDays53Weeks: active,
		Peak:              projectPeak(cal, unavailable),
		Cost:              projectCost(sum.All, unavailable),
		Cells:             projectCells(cal, unavailable),
		Agents:            projectAgents(sum, unavailable),
	}
	return card
}

func quantity(raw int64, unavailable bool) Quantity {
	if unavailable {
		return Quantity{Raw: raw, Display: emDash}
	}
	return Quantity{Raw: raw, Display: FormatCompact(raw)}
}

func projectPeak(cal metric.Calendar, unavailable bool) Peak {
	st := cal.All.Stats
	if unavailable || st.PeakDate == "" || st.PeakTotal <= 0 {
		return Peak{TokensDisplay: emDash}
	}
	return Peak{
		Available:     true,
		Date:          st.PeakDate,
		TokensRaw:     st.PeakTotal,
		TokensDisplay: FormatCompact(st.PeakTotal),
		VisibleInWall: st.PeakDate >= cal.WindowFrom && st.PeakDate <= cal.WindowTo,
	}
}

func projectCost(all metric.Slice, unavailable bool) Cost {
	v := metric.View(all)
	st := v.CostStatus
	if st == "" {
		st = price.StatusUnavailable
	}
	if unavailable {
		return Cost{Status: st, Display: emDash}
	}
	if v.CostUSD == "" {
		return Cost{Status: st, Display: emDash}
	}
	return Cost{Status: st, Display: v.CostUSD}
}

func projectWindow(days []metric.Day, from, to string) (total int64, active int) {
	for _, d := range days {
		if d.Date < from || d.Date > to {
			continue
		}
		total = satAdd(total, d.Total)
		if d.Total > 0 {
			active++
		}
	}
	return total, active
}

func projectCells(cal metric.Calendar, unavailable bool) []Cell {
	out := make([]Cell, 0, WallDays)
	start, err := time.Parse("2006-01-02", cal.WindowFrom)
	if err != nil || cal.WindowFrom == "" {
		return out
	}
	byDate := make(map[string]metric.Day, len(cal.All.Days))
	for _, d := range cal.All.Days {
		byDate[d.Date] = d
	}
	to := cal.WindowTo
	for i := 0; i < WallDays; i++ {
		date := start.AddDate(0, 0, i).Format("2006-01-02")
		c := Cell{Date: date, Level: 0, TokensDisplay: emDash}
		if to != "" && date > to {
			c.State = CellFuture
			out = append(out, c)
			continue
		}
		if unavailable {
			c.State = CellUnknown
			out = append(out, c)
			continue
		}
		if d, ok := byDate[date]; ok && d.Total > 0 {
			c.State = CellActive
			c.Level = d.Level
			c.TokensRaw = d.Total
			c.TokensDisplay = FormatCompact(d.Total)
			out = append(out, c)
			continue
		}
		c.State = CellEmpty
		c.TokensDisplay = "0"
		out = append(out, c)
	}
	return out
}

func projectAgents(sum metric.Summary, unavailable bool) []Agent {
	if unavailable {
		return nil
	}
	src := append([]metric.Slice(nil), sum.BySource...)
	var rows []metric.Slice
	for _, s := range src {
		if s.Total() == 0 {
			continue
		}
		rows = append(rows, s)
	}
	sort.SliceStable(rows, func(i, j int) bool {
		ti, tj := rows[i].Total(), rows[j].Total()
		if ti != tj {
			return ti > tj
		}
		return rows[i].ID < rows[j].ID
	})
	all := sum.All.Total()
	var out []Agent
	var rest int64
	for i, s := range rows {
		if i < 3 {
			out = append(out, agentFrom(s.ID, s.Label, s.Total(), all, false))
			continue
		}
		rest = satAdd(rest, s.Total())
	}
	if rest > 0 {
		out = append(out, agentFrom(agentRestID, agentRestLabel, rest, all, true))
	}
	return out
}

func agentFrom(id, label string, part, all int64, rest bool) Agent {
	ratio := 0.0
	if all > 0 {
		ratio = float64(part) / float64(all)
	}
	return Agent{
		ID:            id,
		Label:         label,
		TokensRaw:     part,
		TokensDisplay: FormatCompact(part),
		ShareText:     metric.FormatShare(part, all),
		ShareRatio:    ratio,
		Rest:          rest,
	}
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

// FormatCompact renders n as a card-only K/M/B/T figure. Canonical raw
// totals stay int64; this is display only. Negatives become "—".
func FormatCompact(n int64) string {
	if n < 0 {
		return emDash
	}
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	switch {
	case n >= 1_000_000_000_000:
		return formatScaled(n, 1_000_000_000_000, "T")
	case n >= 1_000_000_000:
		return formatScaled(n, 1_000_000_000, "B")
	case n >= 1_000_000:
		return formatScaled(n, 1_000_000, "M")
	default:
		return formatScaled(n, 1_000, "K")
	}
}

func formatScaled(n, div int64, suffix string) string {
	whole := n / div
	rem := n % div
	frac := (rem*100 + div/2) / div
	if frac >= 100 {
		whole++
		frac = 0
	}
	if whole >= 1000 && suffix != "T" {
		return FormatCompact(whole * div)
	}
	if frac == 0 {
		return strconv.FormatInt(whole, 10) + suffix
	}
	s := strconv.FormatInt(whole, 10) + "." + twoDigits(frac)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s + suffix
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
