// Package community is the anonymous aggregate Community Rank layer.
//
// Local usage stays on the machine. This package ranks self-reported daily
// token totals among whereToken participants. It is not a global developer
// rank and not an audited leaderboard.
package community

import (
	"fmt"
	"strings"
)

const (
	MetricTokens = "tokens"
	MetricCost   = "estimated_cost"

	PeriodToday = "today"
	PeriodAll   = "all"

	StatusOK                       = "ok"
	StatusUnavailable              = "unavailable"
	StatusNotRanked                = "not_ranked"
	StatusInsufficientParticipants = "insufficient_participants"
	StatusOptedOut                 = "opted_out"
	StatusOffline                  = "offline"
	StatusDisabled                 = "disabled"
	StatusNoUsage                  = "no_usage"
	StatusServiceUnconfigured      = "service_unconfigured"
	StatusNetworkError             = "network_error"

	// DefaultMinParticipants is the first-version floor. Below this the
	// product shows "not available yet" instead of "#1 / 3". Twenty is
	// high enough to avoid a three-person podium and low enough that an
	// alpha community can actually see ranks; raise toward 50–100 as
	// participation grows.
	DefaultMinParticipants = 20

	DisclaimerEN = "Community Rank is self-reported anonymous aggregate usage among participants. It is not a global, worldwide, or all-AI-users rank, and not an audited competitive leaderboard."
	DisclaimerZH = "社区排名基于参与用户匿名上报的聚合用量，不是全球、全世界或全体 AI 用户排名，也不是经过审计的竞技排行榜。"
)

// Standing is one period/metric rank. Rank and percentile are omitted when
// the user is not ranked. Zero is never a stand-in for unavailable.
type Standing struct {
	Status       string   `json:"status"`
	Period       string   `json:"period"`
	Metric       string   `json:"metric"`
	Rank         int      `json:"rank,omitempty"`
	Participants int      `json:"participants,omitempty"`
	Percentile   *float64 `json:"percentile,omitempty"`
	TopShare     *float64 `json:"top_share,omitempty"`
	Display      string   `json:"display,omitempty"`
	Note         string   `json:"note,omitempty"`
	SelfReported bool     `json:"self_reported"`
}

// View is what the local CLI, JSON, and dashboard consume. The dashboard
// only displays these objects; it does not sort or rank.
type View struct {
	Enabled      bool     `json:"enabled"`
	Metric       string   `json:"metric"`
	SelfReported bool     `json:"self_reported"`
	Note         string   `json:"note"`
	Today        Standing `json:"today"`
	All          Standing `json:"all"`
}

func EmptyView(status, note string) View {
	if note == "" {
		note = DisclaimerEN
	}
	st := Standing{Status: status, Metric: MetricTokens, Note: note, SelfReported: true}
	today, all := st, st
	today.Period = PeriodToday
	all.Period = PeriodAll
	return View{
		Enabled:      status != StatusOptedOut && status != StatusDisabled && status != StatusServiceUnconfigured,
		Metric:       MetricTokens,
		SelfReported: true,
		Note:         note,
		Today:        today,
		All:          all,
	}
}

// CompetitionRank is the Olympic / "1224" rule: tied scores share the best
// rank and the next rank skips. A=100, B=100, C=80 → #1, #1, #3.
// Rank is 1-based. scores must be the participating values (strictly > 0).
func CompetitionRank(scores []int64, self int64) (rank, n int) {
	n = len(scores)
	if n == 0 {
		return 0, 0
	}
	better := 0
	for _, v := range scores {
		if v > self {
			better++
		}
	}
	return better + 1, n
}

// Percentile is 1 - (rank-1)/n. Rank 1 of N is 1.0. Rank N of N is 1/N.
func Percentile(rank, n int) float64 {
	if rank <= 0 || n <= 0 {
		return 0
	}
	return 1 - float64(rank-1)/float64(n)
}

// TopFraction is rank/n (the "top 4.4%" share). Auxiliary to Display.
func TopFraction(rank, n int) float64 {
	if rank <= 0 || n <= 0 {
		return 0
	}
	return float64(rank) / float64(n)
}

func FormatDisplay(rank, n int) string {
	if rank <= 0 || n <= 0 {
		return ""
	}
	return fmt.Sprintf("#%d / %d", rank, n)
}

func FinishStanding(status, period, metric string, rank, n, minN int) Standing {
	st := Standing{
		Status:       status,
		Period:       period,
		Metric:       metric,
		SelfReported: true,
		Note:         DisclaimerEN,
	}
	if n > 0 {
		st.Participants = n
	}
	switch status {
	case StatusOK:
		if n < minN {
			st.Status = StatusInsufficientParticipants
			st.Note = "Community ranking is not available yet."
			return st
		}
		if rank <= 0 {
			st.Status = StatusNotRanked
			return st
		}
		st.Rank = rank
		st.Display = FormatDisplay(rank, n)
		p := Percentile(rank, n)
		top := TopFraction(rank, n)
		st.Percentile = &p
		st.TopShare = &top
	case StatusInsufficientParticipants:
		st.Note = "Community ranking is not available yet."
	}
	return st
}

func Caption(st Standing) string {
	st = SanitizeStanding(st)
	if st.Display != "" {
		return st.Display
	}
	return "—"
}

// SanitizeStanding drops a zero or missing rank so callers never print
// "#0". Unknown is an em dash via Caption, not a podium place.
func SanitizeStanding(st Standing) Standing {
	switch st.Status {
	case StatusInsufficientParticipants, StatusNotRanked, StatusUnavailable,
		StatusOptedOut, StatusOffline, StatusDisabled, StatusNoUsage,
		StatusServiceUnconfigured, StatusNetworkError:
		st.Rank = 0
		st.Display = ""
		st.Percentile = nil
		st.TopShare = nil
	}
	if st.Rank <= 0 {
		st.Rank = 0
		st.Display = ""
		st.Percentile = nil
		st.TopShare = nil
		if st.Status == StatusOK {
			st.Status = StatusNotRanked
		}
	}
	if st.Display == "#0" || strings.Contains(st.Display, "#0 /") || strings.Contains(st.Display, "#0/") || strings.HasSuffix(st.Display, " #0") {
		st.Display = ""
		st.Rank = 0
		if st.Status == StatusOK {
			st.Status = StatusNotRanked
		}
	}
	st.SelfReported = true
	if st.Note == "" {
		st.Note = DisclaimerEN
	}
	return st
}
