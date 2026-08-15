package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Filter struct {
	Today        bool
	Tool, Vendor string
	Model        string
	Discovered   []metric.Slice
}

type Row struct {
	ID, Label    string
	TotalM       string
	HitRateText  string
	Requests     int64
	UserTurns    int64
	RequestsText string
	TurnsText    string
	Quality      event.Quality
}

type Snapshot struct {
	Period, Scope string
	Total         int64
	TotalM        string
	HitRate       *float64
	HitRateText   string
	MaxStreak     int
	CurrentStreak int
	Requests      int64
	UserTurns     int64
	ShowStreaks   bool
	Tools         []Row
	Vendors       []Row
	Models        []Row
	Notes         []string
	Quality       event.Quality
}

type usageErr struct{ msg string }

func (e usageErr) Error() string { return e.msg }
func (e usageErr) Usage() bool   { return true }

func IsUsage(err error) bool {
	if err == nil {
		return false
	}
	_, ok := err.(usageErr)
	return ok
}

func Build(events []event.UsageEvent, turns []event.TurnEvent, errs []string, f Filter, now time.Time, loc *time.Location) (Snapshot, error) {
	if loc == nil {
		loc = time.Local
	}
	now = now.In(loc)

	if strings.TrimSpace(f.Model) != "" {
		found := false
		for _, e := range events {
			if modelMatch(e.Model, f.Model) {
				found = true
				break
			}
		}
		if !found {
			return Snapshot{}, usageErr{msg: fmt.Sprintf("unknown model %q", f.Model)}
		}
	}

	var fe []event.UsageEvent
	for _, e := range events {
		if keepEvent(e, f, now, loc) {
			fe = append(fe, e)
		}
	}
	srcSeen := map[string]struct{}{}
	for _, e := range fe {
		srcSeen[e.Source] = struct{}{}
	}
	var ft []event.TurnEvent
	for _, t := range turns {
		if keepTurn(t, f, now, loc, srcSeen) {
			ft = append(ft, t)
		}
	}

	sum := metric.Aggregate(fe, ft)
	view := metric.View(sum.All)
	snap := Snapshot{
		Period:        period(f, now),
		Scope:         scope(f),
		Total:         sum.All.Total(),
		TotalM:        view.TotalM,
		HitRate:       view.HitRate,
		HitRateText:   view.HitRateText,
		MaxStreak:     sum.Calendar.All.Stats.LongestStreak,
		CurrentStreak: sum.Calendar.All.Stats.CurrentStreak,
		Requests:      sum.All.Requests,
		UserTurns:     sum.All.UserTurns,
		ShowStreaks:   !f.Today,
		Quality:       sum.All.Quality,
	}

	for _, s := range sum.BySource {
		snap.Tools = append(snap.Tools, rowFrom(s))
	}
	for _, s := range sum.ByVendor {
		snap.Vendors = append(snap.Vendors, rowFrom(s))
	}
	for _, s := range sum.DrillAll.Models {
		snap.Models = append(snap.Models, rowFrom(s))
	}
	if snap.Tools == nil {
		snap.Tools = []Row{}
	}
	if snap.Vendors == nil {
		snap.Vendors = []Row{}
	}
	if snap.Models == nil {
		snap.Models = []Row{}
	}

	snap.Notes = notes(errs, f.Discovered)
	snap.Tools = mergeDiscovered(snap.Tools, f.Discovered, f.Tool, f.Today)
	snap.Vendors = rankVendors(snap.Vendors)
	if unknownVendorTotal(snap.Vendors) > 0 {
		msg := "Unknown 厂家 · 账本没写模型名（Cursor 账号用量常这样）"
		dup := false
		for _, n := range snap.Notes {
			if n == msg {
				dup = true
				break
			}
		}
		if !dup {
			snap.Notes = append(snap.Notes, msg)
		}
	}
	return snap, nil
}

func rowFrom(s metric.Slice) Row {
	v := metric.View(s)
	return Row{
		ID:           s.ID,
		Label:        s.Label,
		TotalM:       v.TotalM,
		HitRateText:  v.HitRateText,
		Requests:     s.Requests,
		UserTurns:    s.UserTurns,
		RequestsText: metric.FormatCount(s.Requests),
		TurnsText:    metric.FormatCount(s.UserTurns),
		Quality:      s.Quality,
	}
}

func period(f Filter, now time.Time) string {
	if f.Today {
		return "今天 " + now.Format("2006-01-02")
	}
	return "有账本以来"
}

func scope(f Filter) string {
	var parts []string
	if f.Tool != "" {
		parts = append(parts, metric.SourceLabel(f.Tool))
	}
	if f.Vendor != "" {
		parts = append(parts, vendor.Label(f.Vendor))
	}
	if f.Model != "" {
		parts = append(parts, f.Model)
	}
	return strings.Join(parts, " · ")
}

func keepEvent(e event.UsageEvent, f Filter, now time.Time, loc *time.Location) bool {
	if f.Tool != "" && e.Source != f.Tool {
		return false
	}
	if f.Vendor != "" && e.Vendor != f.Vendor {
		return false
	}
	if f.Model != "" && !modelMatch(e.Model, f.Model) {
		return false
	}
	if f.Today {
		if e.Timestamp.IsZero() {
			return false
		}
		if e.Timestamp.In(loc).Format("2006-01-02") != now.Format("2006-01-02") {
			return false
		}
	}
	return true
}

func keepTurn(t event.TurnEvent, f Filter, now time.Time, loc *time.Location, srcSeen map[string]struct{}) bool {
	if f.Tool != "" && t.Source != f.Tool {
		return false
	}
	if f.Today {
		if t.Timestamp.IsZero() {
			return false
		}
		if t.Timestamp.In(loc).Format("2006-01-02") != now.Format("2006-01-02") {
			return false
		}
	}
	if f.Vendor != "" || f.Model != "" {
		if _, ok := srcSeen[t.Source]; !ok {
			return false
		}
	}
	return true
}

func modelMatch(have, want string) bool {
	return strings.EqualFold(strings.TrimSpace(have), strings.TrimSpace(want))
}

func notes(errs []string, discovered []metric.Slice) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, msg := range errs {
		msg = Redact(msg)
		id, rest, ok := strings.Cut(msg, ": ")
		label := id
		if ok {
			if l := metric.SourceLabel(id); l != id || id == "claude" || id == "kimi" || id == "trae" || id == "cursor" || id == "codex" || id == "opencode" {
				label = metric.SourceLabel(id)
			}
			msg = label + " · " + rest
		}
		if _, dup := seen[msg]; dup {
			continue
		}
		seen[msg] = struct{}{}
		out = append(out, msg)
	}
	for _, s := range discovered {
		if s.Quality != event.QualityDegraded && s.Quality != event.QualityAbsent {
			continue
		}
		label := s.Label
		if label == "" {
			label = metric.SourceLabel(s.ID)
		}
		var msg string
		switch {
		case s.Quality == event.QualityAbsent && (s.ID == "cursor" || s.ID == "trae"):
			msg = label + " · 本机没有可用的 token 账本"
		case s.Quality == event.QualityDegraded && (s.ID == "cursor" || s.ID == "trae"):
			msg = label + " · token 列不完整（该工具需要已登录）"
		default:
			continue
		}
		if _, dup := seen[label]; dup {
			continue
		}
		// skip stock note if we already have an error for this source
		skip := false
		for _, n := range out {
			if strings.HasPrefix(n, label+" ·") {
				skip = true
				break
			}
		}
		if skip {
			continue
		}
		out = append(out, msg)
	}
	return out
}

func mergeDiscovered(tools []Row, discovered []metric.Slice, toolFilter string, today bool) []Row {
	if today {
		return tools
	}
	have := map[string]struct{}{}
	for _, r := range tools {
		have[r.ID] = struct{}{}
	}
	for _, s := range discovered {
		if toolFilter != "" && s.ID != toolFilter {
			continue
		}
		if _, ok := have[s.ID]; ok {
			continue
		}
		if s.Total() != 0 || s.Requests != 0 || s.UserTurns != 0 {
			continue
		}
		tools = append(tools, rowFrom(s))
		have[s.ID] = struct{}{}
	}
	return tools
}

func rankVendors(rows []Row) []Row {
	var known, unknown []Row
	for _, r := range rows {
		if r.ID == "unknown" {
			unknown = append(unknown, r)
			continue
		}
		known = append(known, r)
	}
	return append(known, unknown...)
}

func unknownVendorTotal(rows []Row) int64 {
	for _, r := range rows {
		if r.ID == "unknown" && r.Requests+r.UserTurns > 0 {
			return 1
		}
		if r.ID == "unknown" && r.TotalM != "" && r.TotalM != "0.00 M" {
			return 1
		}
	}
	return 0
}
