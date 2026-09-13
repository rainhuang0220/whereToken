package card

import (
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

// FromSnapshot projects a public Snapshot onto the legacy 800×576 card DTO.
func FromSnapshot(s publicprofile.Snapshot) PublicCard {
	unavail := s.DataStatus == DataUnavailable
	all := s.Periods.All
	w53 := s.Periods.W53
	c := PublicCard{
		SchemaVersion: SchemaVersion,
		Version:       s.Producer.Version,
		DataStatus:    s.DataStatus,
		AsOfDate:      s.AsOfDate,
		Range: DateRange{
			WeekStart:  s.Activity.WeekStart,
			WindowFrom: s.Activity.From,
			WindowTo:   s.Activity.To,
		},
		AllTimeTokens:     fromComp(all.Totals.Total, unavail),
		Last53WeeksTokens: fromComp(w53.Totals.Total, unavail),
		CurrentStreakDays: countInt(all.CurrentStreak),
		LongestStreakDays: countInt(all.LongestStreak),
		ActiveDays53Weeks: countInt(w53.ActiveDays),
		Peak:              fromPeak(all.Peak, s.Activity.From, s.Activity.To),
		Cost:              fromCost(all.Cost, unavail),
		Cells:             fromCells(s),
		Agents:            fromAgents(all),
	}
	return c
}

func fromComp(n publicprofile.Component, unavail bool) Quantity {
	raw := int64(0)
	if n.Value != nil {
		raw = *n.Value
	}
	if unavail || n.Status == publicprofile.StatusUnavailable {
		return Quantity{Raw: raw, Display: emDash}
	}
	return Quantity{Raw: raw, Display: n.Display}
}

func countInt(c publicprofile.Count) int {
	if c.Value == nil {
		return 0
	}
	return int(*c.Value)
}

func fromPeak(p publicprofile.Peak, from, to string) Peak {
	raw := int64(0)
	if p.Total.Value != nil {
		raw = *p.Total.Value
	}
	if p.Date == "" || raw <= 0 {
		return Peak{TokensDisplay: emDash}
	}
	return Peak{
		Available:     true,
		Date:          p.Date,
		TokensRaw:     raw,
		TokensDisplay: p.Total.Display,
		VisibleInWall: p.Date >= from && p.Date <= to,
	}
}

func fromCost(c *publicprofile.Cost, unavail bool) Cost {
	if c == nil || unavail {
		return Cost{Status: DataUnavailable, Display: emDash}
	}
	return Cost{Status: c.Status, Display: c.Display}
}

func fromCells(s publicprofile.Snapshot) []Cell {
	var ser publicprofile.Series
	for _, it := range s.Activity.Series {
		if it.Dimension == "all" {
			ser = it
			break
		}
	}
	out := make([]Cell, 0, len(s.Activity.Dates))
	for i, date := range s.Activity.Dates {
		c := Cell{Date: date, TokensDisplay: emDash}
		st := CellEmpty
		if i < len(ser.States) {
			st = ser.States[i]
		}
		c.State = st
		if i < len(ser.Levels) {
			c.Level = ser.Levels[i]
		}
		if i < len(ser.Values) {
			c.TokensRaw = ser.Values[i]
			if st == CellActive {
				c.TokensDisplay = FormatCompact(ser.Values[i])
			} else if st == CellEmpty {
				c.TokensDisplay = "0"
			}
		}
		out = append(out, c)
	}
	return out
}

func fromAgents(all publicprofile.Period) []Agent {
	var out []Agent
	var rest int64
	for i, a := range all.ByAgent {
		if i >= 3 {
			if a.Totals.Total.Value != nil {
				rest += *a.Totals.Total.Value
			}
			continue
		}
		raw := int64(0)
		if a.Totals.Total.Value != nil {
			raw = *a.Totals.Total.Value
		}
		out = append(out, Agent{
			ID:            a.ID,
			Label:         a.Label,
			TokensRaw:     raw,
			TokensDisplay: a.Totals.Total.Display,
			ShareText:     a.Share,
			Rest:          false,
		})
	}
	if rest > 0 {
		total := int64(0)
		if all.Totals.Total.Value != nil {
			total = *all.Totals.Total.Value
		}
		out = append(out, Agent{
			ID:            agentRestID,
			Label:         agentRestLabel,
			TokensRaw:     rest,
			TokensDisplay: FormatCompact(rest),
			ShareText:     metric.FormatShare(rest, total),
			ShareRatio:    partRatio(rest, total),
			Rest:          true,
		})
	}
	return out
}

func partRatio(part, all int64) float64 {
	if all <= 0 {
		return 0
	}
	return float64(part) / float64(all)
}
