package report

import (
	"fmt"
	"strings"

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
	b.WriteString(title(snap))
	b.WriteByte('\n')
	if spark := sparkLine(snap, opt.ASCII); spark != "" {
		b.WriteString(spark)
		b.WriteByte('\n')
	}
	b.WriteByte('\n')
	b.WriteString(table.KPIBox(kpiCells(snap), style))
	b.WriteString(legend())
	if len(snap.Tools) > 0 && snap.Scope == "" {
		b.WriteByte('\n')
		b.WriteString(ranked("工具", snap.Tools, true, style, opt.Width))
	}
	if len(snap.Vendors) > 0 {
		b.WriteByte('\n')
		b.WriteString(ranked("厂家", snap.Vendors, false, style, opt.Width))
	}
	if (snap.Scope != "" || !snap.ShowStreaks) && len(snap.Models) > 0 {
		b.WriteByte('\n')
		b.WriteString(ranked("模型", snap.Models, false, style, opt.Width))
	}
	if snap.Scope != "" && len(snap.Tools) > 0 && !allSameTool(snap) {
		b.WriteByte('\n')
		b.WriteString(ranked("工具", snap.Tools, true, style, opt.Width))
	}
	if len(snap.Notes) > 0 {
		b.WriteByte('\n')
		var nb strings.Builder
		nb.WriteString("注\n")
		for _, n := range snap.Notes {
			nb.WriteString("  · ")
			nb.WriteString(n)
			nb.WriteByte('\n')
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

func kpiCells(snap Snapshot) [2][3]table.KPI {
	if snap.ShowStreaks {
		return [2][3]table.KPI{
			{
				{Label: "总用量", Value: snap.TotalM},
				{Label: "命中率", Value: snap.HitRateText},
				{Label: "最长连烧", Value: days(snap.MaxStreak)},
			},
			{
				{Label: "当前连烧", Value: days(snap.CurrentStreak)},
				{Label: "请求", Value: metric.FormatCount(snap.Requests)},
				{Label: "用户回合", Value: metric.FormatCount(snap.UserTurns)},
			},
		}
	}
	return [2][3]table.KPI{
		{
			{Label: "总用量", Value: snap.TotalM},
			{Label: "命中率", Value: snap.HitRateText},
			{Label: "请求", Value: metric.FormatCount(snap.Requests)},
		},
		{
			{Label: "用户回合", Value: metric.FormatCount(snap.UserTurns)},
			{Label: "工具数", Value: metric.FormatCount(int64(len(snap.Tools)))},
			{Label: "厂家数", Value: metric.FormatCount(int64(len(snap.Vendors)))},
		},
	}
}

func days(n int) string {
	return fmt.Sprintf("%s 天", metric.FormatCount(int64(n)))
}

func sparkLine(snap Snapshot, ascii bool) string {
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
	if ascii {
		return "7d  " + bar
	}
	return "近7日  " + bar
}

func legend() string {
	return "  合计 = 未命中 + 缓存读 + 缓存写 + 输出。命中率不含输出。\n"
}

func ranked(kind string, rows []Row, withTurns bool, style table.BoxStyle, maxWidth int) string {
	headers := []string{kind, "合计", "占比", "命中率", "请求"}
	align := []table.Align{table.AlignLeft, table.AlignRight, table.AlignRight, table.AlignRight, table.AlignRight}
	if withTurns {
		headers = append(headers, "回合")
		align = append(align, table.AlignRight)
	}
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
		line := []string{label, r.TotalM, share, r.HitRateText, r.RequestsText}
		if withTurns {
			line = append(line, r.TurnsText)
		}
		body = append(body, line)
	}
	out := table.FitRankedTable(headers, body, align, style, maxWidth)
	if len(rows) > capN {
		out += fmt.Sprintf("+ %d 行\n", len(rows)-capN)
	}
	return out
}

func allSameTool(snap Snapshot) bool {
	if snap.Scope == "" || len(snap.Tools) == 0 {
		return true
	}
	return len(snap.Tools) == 1
}
