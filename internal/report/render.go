package report

import (
	"fmt"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/community"
	"github.com/rainhuang0220/whereToken/internal/metric"
	"github.com/rainhuang0220/whereToken/internal/table"
)

type Options struct {
	ASCII bool
	Color bool
	Width int
}

func Render(snap Snapshot, opt Options) string {
	style := table.BoxUnicode
	if opt.ASCII {
		style = table.BoxASCII
	}
	var b strings.Builder
	ttl := title(snap)
	mid := ""
	pose := table.PoseGrin
	if isColdKiln(snap) {
		pose = table.PoseBlink
		mid = "窑里还是冷的"
	} else if spark := sparkLine(snap, opt.ASCII, false); spark != "" {
		mid = spark
	}
	b.WriteString(table.SpriteScene(pose, ttl, mid, "", opt.ASCII, opt.Color))
	if banner := offlineBanner(snap); banner != "" {
		writeWrapped(&b, table.Dim(banner, opt.Color), opt.Width)
	}
	b.WriteByte('\n')
	b.WriteString(table.FitKPIBox(kpiCells(snap, opt.Color), style, opt.Width))
	leg := legend(opt.Width)
	if opt.Color {
		leg = table.Dim(strings.TrimRight(leg, "\n"), true) + "\n"
	}
	b.WriteString(leg)
	if len(snap.Tools) > 0 && snap.Scope == "" {
		b.WriteByte('\n')
		b.WriteString(ranked("工具", snap.Tools, true, style, opt.Width, opt.Color))
	}
	if len(snap.Vendors) > 0 {
		b.WriteByte('\n')
		b.WriteString(ranked("厂家", snap.Vendors, false, style, opt.Width, opt.Color))
	}
	if (snap.Scope != "" || !snap.ShowStreaks) && anyPositiveTotal(snap.Models) {
		b.WriteByte('\n')
		b.WriteString(ranked("模型", snap.Models, false, style, opt.Width, opt.Color))
	}
	if snap.Scope != "" && len(snap.Tools) > 0 && !allSameTool(snap) {
		b.WriteByte('\n')
		b.WriteString(ranked("工具", snap.Tools, true, style, opt.Width, opt.Color))
	}
	footnoteNotes := footnotes(snap)
	if len(footnoteNotes) > 0 {
		b.WriteByte('\n')
		var nb strings.Builder
		nb.WriteString("注\n")
		for _, n := range footnoteNotes {
			writeWrapped(&nb, "  · "+n, opt.Width)
		}
		block := nb.String()
		if opt.Color {
			block = table.Dim(block, true)
		}
		b.WriteString(block)
	}
	return b.String()
}

func title(snap Snapshot) string {
	parts := []string{"whereToken"}
	if snap.Scope != "" {
		parts = append(parts, snap.Scope)
	}
	parts = append(parts, snap.Period)
	return strings.Join(parts, " · ")
}

func turnKPI(snap Snapshot) string {
	if snap.HideTurns {
		return "—"
	}
	return metric.FormatCount(snap.UserTurns)
}

func kpiCells(snap Snapshot, color bool) [2][]table.KPI {
	hit := table.PaintHit(snap.HitRateText, color)
	total := table.Bold(snap.TotalM, color)
	cost := costKPI(snap)
	rank := community.Caption(rankStanding(snap))
	if snap.ShowStreaks {
		return [2][]table.KPI{
			{
				{Label: "总用量", Value: total},
				{Label: "命中率", Value: hit},
				{Label: "最长连烧", Value: days(snap.MaxStreak)},
				{Label: "当日用量", Value: snap.TodayM},
				{Label: "估价", Value: cost},
			},
			{
				{Label: "当前连烧", Value: days(snap.CurrentStreak)},
				{Label: "请求", Value: metric.FormatCount(snap.Requests)},
				{Label: "用户回合", Value: turnKPI(snap)},
				{Label: "单日最高", Value: snap.PeakDayM},
				{Label: "排名", Value: rank},
			},
		}
	}
	return [2][]table.KPI{
		{
			{Label: "总用量", Value: total},
			{Label: "命中率", Value: hit},
			{Label: "请求", Value: metric.FormatCount(snap.Requests)},
			{Label: "估价", Value: cost},
		},
		{
			{Label: "用户回合", Value: turnKPI(snap)},
			{Label: "工具数", Value: metric.FormatCount(int64(len(snap.Tools)))},
			{Label: "厂家数", Value: metric.FormatCount(int64(len(snap.Vendors)))},
			{Label: "排名", Value: rank},
		},
	}
}

func costKPI(snap Snapshot) string {
	if snap.CostUSD != "" {
		return snap.CostUSD
	}
	return "—"
}

func rankStanding(snap Snapshot) community.Standing {
	st := snap.Community.Today
	if snap.RankPeriod == community.PeriodAll {
		st = snap.Community.All
	}
	return community.SanitizeStanding(st)
}

func days(n int) string {
	return fmt.Sprintf("%s 天", metric.FormatCount(int64(n)))
}

func rowCost(r Row) string {
	if r.CostStatus == "unavailable" || r.CostUSD == "" {
		return "—"
	}
	return r.CostUSD
}

func sparkLine(snap Snapshot, ascii, color bool) string {
	if !snap.ShowStreaks || len(snap.Last7) == 0 {
		return ""
	}
	var max int64
	for _, v := range snap.Last7 {
		if v > max {
			max = v
		}
	}
	if max == 0 {
		return ""
	}
	bar := table.Spark(snap.Last7, ascii)
	label := "近7日  "
	if ascii {
		label = "7d  "
	}
	return table.Dim(label, color) + table.Lemon(bar, color)
}

func legend(width int) string {
	wide := "  合计 = 未命中 + 缓存读 + 缓存写 + 输出。命中率不含输出。"
	if width <= 0 || table.DisplayWidth(wide) <= width {
		return wide + "\n"
	}
	return "  合计=未命中+缓存读+缓存写+输出。\n  命中率不含输出。\n"
}

func ranked(kind string, rows []Row, withTurns bool, style table.BoxStyle, maxWidth int, color bool) string {
	headers := []string{kind, "合计", "占比", "命中率", "请求"}
	align := []table.Align{table.AlignLeft, table.AlignRight, table.AlignRight, table.AlignRight, table.AlignRight}
	if withTurns {
		headers = append(headers, "回合")
		align = append(align, table.AlignRight)
	}
	headers = append(headers, "估价")
	align = append(align, table.AlignRight)
	const capN = 12
	n := len(rows)
	if n > capN {
		n = capN
	}
	body := make([][]string, 0, n)
	for i := 0; i < n; i++ {
		r := rows[i]
		label := r.Label
		if kind == "模型" {
			label = table.Truncate(label, 36)
		}
		share := r.ShareText
		if share == "" {
			share = "—"
		}
		line := []string{label, r.TotalM, share, table.PaintHit(r.HitRateText, color), r.RequestsText}
		if withTurns {
			line = append(line, r.TurnsText)
		}
		line = append(line, rowCost(r))
		body = append(body, line)
	}
	if color {
		for i, h := range headers {
			headers[i] = table.Lemon(h, true)
		}
	}
	out := table.FitRankedTable(headers, body, align, style, maxWidth)
	if len(rows) > capN {
		out += fmt.Sprintf("+ %d 行\n", len(rows)-capN)
	}
	return out
}

func isColdKiln(snap Snapshot) bool {
	if snap.Total != 0 || snap.Requests != 0 || snap.UserTurns != 0 {
		return false
	}
	for _, n := range snap.Notes {
		if strings.Contains(n, "本机没有找到账本") {
			return true
		}
	}
	return len(snap.Tools) == 0 && len(snap.Vendors) == 0
}

func anyPositiveTotal(rows []Row) bool {
	for _, r := range rows {
		if r.Total != 0 {
			return true
		}
	}
	return false
}

func allSameTool(snap Snapshot) bool {
	if snap.Scope == "" || len(snap.Tools) == 0 {
		return true
	}
	return len(snap.Tools) == 1
}

func writeWrapped(b *strings.Builder, s string, width int) {
	for i, line := range table.Wrap(s, width) {
		if i > 0 && (width <= 0 || table.DisplayWidth(line)+2 <= width) {
			b.WriteString("  ")
		}
		b.WriteString(line)
		b.WriteByte('\n')
	}
}

func offlineBanner(snap Snapshot) string {
	for _, n := range snap.Notes {
		if strings.HasPrefix(n, "offline ·") {
			return n
		}
	}
	return ""
}

func footnotes(snap Snapshot) []string {
	out := make([]string, 0, len(snap.Notes))
	for _, n := range snap.Notes {
		if strings.HasPrefix(n, "offline ·") {
			continue
		}
		out = append(out, n)
	}
	return appendRankNotes(out, snap)
}

func appendRankNotes(notes []string, snap Snapshot) []string {
	st := community.SanitizeStanding(rankStanding(snap))
	switch st.Status {
	case community.StatusOK:
		period := "今日"
		if snap.RankPeriod == community.PeriodAll {
			period = "累计"
		}
		return append(notes, "社区排名 "+st.Display+" · "+period+" · 匿名聚合，不是审计榜")
	case community.StatusInsufficientParticipants:
		return append(notes, "社区排名暂不可用 · 参与者还不够")
	case community.StatusOptedOut, community.StatusDisabled:
		return append(notes, "社区排名已关闭 · wheretoken community on 重新参加")
	case community.StatusOffline:
		return append(notes, "社区排名未上传 · --offline")
	case community.StatusNetworkError:
		return append(notes, "社区排名暂不可用 · 服务连不上")
	default:
		return notes
	}
}
