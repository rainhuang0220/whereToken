package report

import (
	"encoding/json"
	"io"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/community"
)

type jsonRow struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	Total       int64  `json:"total"`
	TotalM      string `json:"total_m"`
	Share       string `json:"share,omitempty"`
	HitRateText string `json:"hit_rate_text"`
	Requests    int64  `json:"requests"`
	UserTurns   int64  `json:"user_turns,omitempty"`
	CostStatus  string `json:"cost_status,omitempty"`
	CostUSD     string `json:"cost_usd,omitempty"`
}

type jsonSnap struct {
	Schema            int             `json:"schema"`
	Period            string          `json:"period"`
	Scope             string          `json:"scope,omitempty"`
	Total             int64           `json:"total"`
	TotalM            string          `json:"total_m"`
	HitRate           *float64        `json:"hit_rate"`
	HitRateText       string          `json:"hit_rate_text"`
	MaxStreakDays     *int            `json:"max_streak_days,omitempty"`
	CurrentStreakDays *int            `json:"current_streak_days,omitempty"`
	Requests          int64           `json:"requests"`
	UserTurns         int64           `json:"user_turns"`
	HideTurns         bool            `json:"hide_turns,omitempty"`
	Last7             []int64         `json:"last_7d,omitempty"`
	Today             int64           `json:"today,omitempty"`
	TodayM            string          `json:"today_m,omitempty"`
	PeakDay           int64           `json:"peak_day,omitempty"`
	PeakDayM          string          `json:"peak_day_m,omitempty"`
	Tools             []jsonRow       `json:"tools"`
	Vendors           []jsonRow       `json:"vendors"`
	Models            []jsonRow       `json:"models,omitempty"`
	Notes             []string        `json:"notes"`
	CostStatus        string          `json:"cost_status,omitempty"`
	CostUSD           string          `json:"cost_usd,omitempty"`
	Community         *community.View `json:"community,omitempty"`
}

func WriteJSON(w io.Writer, snap Snapshot) error {
	out := jsonSnap{
		Schema:      1,
		Period:      snap.Period,
		Scope:       snap.Scope,
		Total:       snap.Total,
		TotalM:      snap.TotalM,
		HitRate:     snap.HitRate,
		HitRateText: snap.HitRateText,
		Requests:    snap.Requests,
		UserTurns:   snap.UserTurns,
		HideTurns:   snap.HideTurns,
		Tools:       jsonRows(snap.Tools, true),
		Vendors:     jsonRows(snap.Vendors, false),
		Notes:       snap.Notes,
		CostStatus:  snap.CostStatus,
		CostUSD:     omitZeroUSD(snap.CostUSD),
	}
	if snap.CostStatus == "unavailable" {
		out.CostUSD = ""
	}
	if snap.ShowStreaks {
		max := snap.MaxStreak
		cur := snap.CurrentStreak
		out.MaxStreakDays = &max
		out.CurrentStreakDays = &cur
		out.Last7 = snap.Last7
		out.Today = snap.TodayTotal
		out.TodayM = snap.TodayM
		out.PeakDay = snap.PeakDay
		out.PeakDayM = snap.PeakDayM
	}
	if snap.Scope != "" || !snap.ShowStreaks {
		out.Models = jsonRows(snap.Models, false)
	}
	if out.Notes == nil {
		out.Notes = []string{}
	}
	if snap.Community.Today.Status != "" || snap.Community.All.Status != "" {
		view := snap.Community
		view.Today = community.SanitizeStanding(view.Today)
		view.All = community.SanitizeStanding(view.All)
		out.Community = &view
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
}

// omitZeroUSD drops a rounding-to-zero estimate. Unknown is omitted, never $0.
func omitZeroUSD(usd string) string {
	switch strings.TrimSpace(usd) {
	case "", "$0.0000", "-$0.0000", "$0.00", "-$0.00", "$0", "-$0":
		return ""
	default:
		return usd
	}
}

func jsonRows(rows []Row, turns bool) []jsonRow {
	if rows == nil {
		return []jsonRow{}
	}
	out := make([]jsonRow, 0, len(rows))
	for _, r := range rows {
		item := jsonRow{
			ID:          r.ID,
			Label:       r.Label,
			Total:       r.Total,
			TotalM:      r.TotalM,
			Share:       r.ShareText,
			HitRateText: r.HitRateText,
			Requests:    r.Requests,
			CostStatus:  r.CostStatus,
			CostUSD:     omitZeroUSD(r.CostUSD),
		}
		if r.CostStatus == "unavailable" {
			item.CostUSD = ""
		}
		if turns {
			item.UserTurns = r.UserTurns
		}
		out = append(out, item)
	}
	return out
}
