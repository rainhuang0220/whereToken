package report

import (
	"encoding/json"
	"io"
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
}

type jsonSnap struct {
	Schema            int       `json:"schema"`
	Period            string    `json:"period"`
	Scope             string    `json:"scope,omitempty"`
	Total             int64     `json:"total"`
	TotalM            string    `json:"total_m"`
	HitRate           *float64  `json:"hit_rate"`
	HitRateText       string    `json:"hit_rate_text"`
	MaxStreakDays     int       `json:"max_streak_days,omitempty"`
	CurrentStreakDays int       `json:"current_streak_days,omitempty"`
	Requests          int64     `json:"requests"`
	UserTurns         int64     `json:"user_turns"`
	Last7             []int64   `json:"last_7d,omitempty"`
	Tools             []jsonRow `json:"tools"`
	Vendors           []jsonRow `json:"vendors"`
	Models            []jsonRow `json:"models,omitempty"`
	Notes             []string  `json:"notes"`
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
		Tools:       jsonRows(snap.Tools, true),
		Vendors:     jsonRows(snap.Vendors, false),
		Notes:       snap.Notes,
	}
	if snap.ShowStreaks {
		out.MaxStreakDays = snap.MaxStreak
		out.CurrentStreakDays = snap.CurrentStreak
		out.Last7 = snap.Last7
	}
	if snap.Scope != "" || !snap.ShowStreaks {
		out.Models = jsonRows(snap.Models, false)
	}
	if out.Notes == nil {
		out.Notes = []string{}
	}
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(out)
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
		}
		if turns {
			item.UserTurns = r.UserTurns
		}
		out = append(out, item)
	}
	return out
}
