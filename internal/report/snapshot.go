package report

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

type Filter struct {
	Today        bool
	Days         int
	From, To     time.Time
	Period       string
	Tool, Vendor string
	Model        string
	Discovered   []metric.Slice
}

type Row struct {
	ID, Label    string
	Total        int64
	TotalM       string
	ShareText    string
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
	HideTurns     bool
	Last7         []int64
	TodayTotal    int64
	TodayM        string
	PeakDay       int64
	PeakDayM      string
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
			if len(events) == 0 {
				// Empty machine: copying --model=k3 from --help is not a usage error.
			} else {
				msg := fmt.Sprintf("unknown model %q", f.Model)
				if s := suggestModel(f.Model, events); s != "" {
					msg += fmt.Sprintf(" (did you mean %q?)", s)
				}
				return Snapshot{}, usageErr{msg: msg}
			}
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
	cal := metric.BuildCalendar(fe, loc, now)
	snap := Snapshot{
		Period:        period(f, now),
		Scope:         scope(f),
		Total:         sum.All.Total(),
		TotalM:        view.TotalM,
		HitRate:       view.HitRate,
		HitRateText:   view.HitRateText,
		MaxStreak:     cal.All.Stats.LongestStreak,
		CurrentStreak: cal.All.Stats.CurrentStreak,
		Requests:      sum.All.Requests,
		UserTurns:     sum.All.UserTurns,
		ShowStreaks:   !windowed(f),
		HideTurns:     f.Model != "",
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
	snap.Tools = mergeDiscovered(snap.Tools, f.Discovered, f.Tool, windowed(f))
	snap.Vendors = rankVendors(snap.Vendors)
	applyShares(snap.Tools, snap.Total)
	applyShares(snap.Vendors, snap.Total)
	applyShares(snap.Models, snap.Total)
	if snap.ShowStreaks {
		snap.Last7 = metric.LastNDailyTotals(cal.All.Days, now, 7)
		if n := len(snap.Last7); n > 0 {
			snap.TodayTotal = snap.Last7[n-1]
		}
		snap.PeakDay = cal.All.Stats.PeakTotal
	}
	snap.TodayM = metric.FormatM(snap.TodayTotal)
	snap.PeakDayM = metric.FormatM(snap.PeakDay)
	if unknownVendorTotal(snap.Vendors) > 0 {
		snap.Notes = appendUniqueNote(snap.Notes, "未知厂家 · 账本没写模型名（Cursor 账号用量常这样）")
	}
	if (snap.Scope != "" || !snap.ShowStreaks) && tokenlessModels(snap.Models) && !(snap.Total == 0 && snap.Requests > 0) {
		snap.Notes = appendUniqueNote(snap.Notes, "有的模型只有请求次数、账本没写 token（Cursor 会话标题常这样）")
	}
	snap.Notes = pruneNotes(snap.Notes, f, snap.Tools)
	for _, r := range snap.Tools {
		if r.ID == "cursor" && r.Quality == event.QualityAuthoritative && r.Total != 0 {
			snap.Notes = appendUniqueNote(snap.Notes, "Cursor · token 列是近 53 周账号用量，请求/回合仍是本机全部会话")
			break
		}
	}
	if snap.Total == 0 && snap.Requests > 0 {
		snap.Notes = appendUniqueNote(snap.Notes, "总用量是 0 但有请求：本机账本只记了次数（Cursor 要登录，或不要 --offline）")
	}
	snap.Notes = appendEmptyViewNotes(snap.Notes, snap, f)
	return snap, nil
}

func appendEmptyViewNotes(notes []string, snap Snapshot, f Filter) []string {
	if snap.Total != 0 || snap.Requests != 0 || snap.UserTurns != 0 {
		return notes
	}
	if windowed(f) {
		unscoped := f.Tool == "" && f.Vendor == "" && f.Model == ""
		if unscoped && len(snap.Tools) == 0 && !discoveredHasUsage(f.Discovered, "", true) {
			return appendUniqueNote(notes, "本机没有找到账本。Claude / Kimi / Codex / OpenCode 有本地记录才会出数；Cursor / Trae 需要已登录。")
		}
		msg := emptyWindowNote(f)
		if discoveredHasUsage(f.Discovered, f.Tool, unscoped) {
			if f.Today {
				msg += "有账本以来请去掉 --today。"
			} else {
				msg += "有账本以来请去掉时间范围。"
			}
		}
		return appendUniqueNote(notes, msg)
	}
	if f.Tool != "" && len(snap.Tools) == 0 {
		return appendUniqueNote(notes, metric.SourceLabel(f.Tool)+" 在这台机器上没有找到账本。")
	}
	if f.Vendor != "" && len(snap.Vendors) == 0 {
		return appendUniqueNote(notes, vendor.Label(f.Vendor)+" 在这台机器上没有找到账本。")
	}
	if f.Model != "" && len(snap.Models) == 0 {
		return appendUniqueNote(notes, "本机账本里没有模型 "+f.Model+"。")
	}
	if f.Tool == "" && f.Vendor == "" && f.Model == "" && len(snap.Tools) == 0 && len(notes) == 0 {
		return appendUniqueNote(notes, "本机没有找到账本。Claude / Kimi / Codex / OpenCode 有本地记录才会出数；Cursor / Trae 需要已登录。")
	}
	return notes
}

func discoveredHasUsage(discovered []metric.Slice, tool string, unscoped bool) bool {
	if tool == "" && !unscoped {
		return false
	}
	for _, s := range discovered {
		if tool != "" && s.ID != tool {
			continue
		}
		if s.Total() != 0 || s.Requests != 0 || s.UserTurns != 0 {
			return true
		}
	}
	return false
}

func appendUniqueNote(notes []string, msg string) []string {
	for _, n := range notes {
		if n == msg {
			return notes
		}
	}
	return append(notes, msg)
}

func windowed(f Filter) bool {
	return f.Today || f.Days > 0 || !f.From.IsZero() || !f.To.IsZero()
}

func emptyWindowNote(f Filter) string {
	if f.Today {
		return "今天还没有用量。"
	}
	if f.Days > 0 {
		return "这段时间还没有用量。"
	}
	return "这段时间还没有用量。"
}

func pruneNotes(notes []string, f Filter, tools []Row) []string {
	if !windowed(f) && f.Tool == "" && f.Vendor == "" && f.Model == "" {
		return notes
	}
	keep := map[string]struct{}{}
	add := func(s string) {
		s = strings.TrimSpace(s)
		if s == "" {
			return
		}
		keep[s] = struct{}{}
		keep[strings.ToLower(s)] = struct{}{}
	}
	add(f.Tool)
	if f.Tool != "" {
		add(metric.SourceLabel(f.Tool))
	}
	for _, r := range tools {
		add(r.ID)
		add(r.Label)
	}
	var out []string
	for _, n := range notes {
		label, _, ok := strings.Cut(n, " · ")
		if !ok {
			out = append(out, n)
			continue
		}
		if strings.Contains(label, "厂家") {
			out = append(out, n)
			continue
		}
		if strings.EqualFold(label, "offline") {
			out = append(out, n)
			continue
		}
		if strings.Contains(n, "登录") || strings.Contains(n, "加密存储") {
			if f.Tool != "" {
				if strings.EqualFold(label, f.Tool) || strings.EqualFold(label, metric.SourceLabel(f.Tool)) {
					out = append(out, n)
				}
				continue
			}
			if windowed(f) && len(tools) == 0 {
				out = append(out, n)
			}
			continue
		}
		if _, hit := keep[label]; hit {
			out = append(out, n)
			continue
		}
		if _, hit := keep[strings.ToLower(label)]; hit {
			out = append(out, n)
			continue
		}
	}
	return out
}

func tokenlessModels(rows []Row) bool {
	for _, r := range rows {
		if r.Total == 0 && r.Requests > 0 {
			return true
		}
	}
	return false
}

func rowFrom(s metric.Slice) Row {
	v := metric.View(s)
	return Row{
		ID:           s.ID,
		Label:        s.Label,
		Total:        s.Total(),
		TotalM:       v.TotalM,
		HitRateText:  v.HitRateText,
		Requests:     s.Requests,
		UserTurns:    s.UserTurns,
		RequestsText: metric.FormatCount(s.Requests),
		TurnsText:    metric.FormatCount(s.UserTurns),
		Quality:      s.Quality,
	}
}

func applyShares(rows []Row, total int64) {
	for i := range rows {
		rows[i].ShareText = metric.FormatShare(rows[i].Total, total)
	}
}

func period(f Filter, now time.Time) string {
	if f.Period != "" {
		return f.Period
	}
	if f.Today {
		return "今天 " + now.Format("2006-01-02")
	}
	if f.Days > 0 {
		return "近 " + strconv.Itoa(f.Days) + " 天"
	}
	if !f.From.IsZero() && !f.To.IsZero() {
		return f.From.Format("2006-01-02") + " … " + f.To.Add(-time.Nanosecond).Format("2006-01-02")
	}
	if !f.From.IsZero() {
		return f.From.Format("2006-01-02") + " 起"
	}
	if !f.To.IsZero() {
		return "至 " + f.To.Add(-time.Nanosecond).Format("2006-01-02")
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
	return inTime(e.Timestamp, f, now, loc)
}

func inTime(ts time.Time, f Filter, now time.Time, loc *time.Location) bool {
	if f.Today {
		if ts.IsZero() {
			return false
		}
		return ts.In(loc).Format("2006-01-02") == now.Format("2006-01-02")
	}
	if !f.From.IsZero() {
		if ts.IsZero() || ts.In(loc).Before(f.From) {
			return false
		}
	}
	if !f.To.IsZero() {
		if ts.IsZero() || !ts.In(loc).Before(f.To) {
			return false
		}
	}
	return true
}

func keepTurn(t event.TurnEvent, f Filter, now time.Time, loc *time.Location, srcSeen map[string]struct{}) bool {
	if f.Tool != "" && t.Source != f.Tool {
		return false
	}
	if !inTime(t.Timestamp, f, now, loc) {
		return false
	}
	if f.Model != "" {
		return false
	}
	if f.Vendor != "" {
		if _, ok := srcSeen[t.Source]; !ok {
			return false
		}
	}
	return true
}

func modelMatch(have, want string) bool {
	have = strings.TrimSpace(have)
	want = strings.TrimSpace(want)
	if have == "" || want == "" {
		return false
	}
	if strings.EqualFold(have, want) {
		return true
	}
	if i := strings.LastIndex(have, "/"); i >= 0 {
		if strings.EqualFold(have[i+1:], want) {
			return true
		}
	}
	return false
}

func notes(errs []string, discovered []metric.Slice) []string {
	var out []string
	seen := map[string]struct{}{}
	for _, msg := range errs {
		msg = Redact(msg)
		id, rest, ok := strings.Cut(msg, ": ")
		label := id
		if ok {
			if l := metric.SourceLabel(id); l != id || id == "claude" || id == "kimi" || id == "grok" || id == "trae" || id == "cursor" || id == "codex" || id == "opencode" {
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

func mergeDiscovered(tools []Row, discovered []metric.Slice, toolFilter string, skip bool) []Row {
	if skip {
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
