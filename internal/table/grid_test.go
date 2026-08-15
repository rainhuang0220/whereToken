package table

import (
	"strings"
	"testing"
)

func TestKPIBoxUnicodeThreeByTwo(t *testing.T) {
	cells := [2][3]KPI{
		{{Label: "总用量", Value: "360.11 M"}, {Label: "命中率", Value: "70.2%"}, {Label: "最长连烧", Value: "14 天"}},
		{{Label: "当前连烧", Value: "3 天"}, {Label: "请求", Value: "12,048"}, {Label: "用户回合", Value: "8,901"}},
	}
	got := KPIBox(cells, BoxUnicode)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) != 7 {
		t.Fatalf("lines=%d\n%s", len(lines), got)
	}
	if !strings.HasPrefix(lines[0], "┌") || !strings.HasSuffix(lines[0], "┐") {
		t.Fatalf("top %q", lines[0])
	}
	if !strings.Contains(lines[1], "总用量") || !strings.Contains(lines[1], "命中率") {
		t.Fatalf("labels %q", lines[1])
	}
	if !strings.Contains(lines[2], "360.11 M") || !strings.Contains(lines[2], "70.2%") {
		t.Fatalf("values %q", lines[2])
	}
	idx := strings.Index(lines[2], "360.11 M")
	if idx < 1 || lines[2][idx-1] != ' ' {
		t.Fatalf("value not right-aligned: %q", lines[2])
	}
	w := DisplayWidth(lines[0])
	for i, line := range lines {
		if DisplayWidth(line) != w {
			t.Fatalf("line %d width %d want %d\n%q", i, DisplayWidth(line), w, line)
		}
	}
}

func TestKPIBoxASCIIFallback(t *testing.T) {
	cells := [2][3]KPI{
		{{Label: "total", Value: "1.00 M"}, {Label: "hit", Value: "—"}, {Label: "streak", Value: "0 days"}},
		{{Label: "current", Value: "0 days"}, {Label: "req", Value: "0"}, {Label: "turns", Value: "0"}},
	}
	got := KPIBox(cells, BoxASCII)
	if strings.ContainsAny(got, "┌┐└┘├┤┬┴┼─│") {
		t.Fatalf("unicode leaked:\n%s", got)
	}
	if !strings.Contains(got, "+") || !strings.Contains(got, "|") {
		t.Fatalf("expected ascii box:\n%s", got)
	}
}

func TestRankedTableAlignsCJK(t *testing.T) {
	got := RankedTable(
		[]string{"工具", "合计", "命中率"},
		[][]string{
			{"Claude Code", "8.10 M", "52.1%"},
			{"Trae", "0.00 M", "—"},
		},
		[]Align{AlignLeft, AlignRight, AlignRight},
		BoxUnicode,
	)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	if len(lines) < 3 {
		t.Fatalf("got %s", got)
	}
	w := DisplayWidth(lines[0])
	for i, line := range lines {
		if DisplayWidth(line) != w {
			t.Fatalf("line %d width %d want %d\n%q\n%s", i, DisplayWidth(line), w, line, got)
		}
	}
	if !strings.Contains(got, "Claude Code") || !strings.Contains(got, "Trae") {
		t.Fatalf("%s", got)
	}
}

func TestKPIBoxHugeGroupedMSameWidth(t *testing.T) {
	cells := [2][3]KPI{
		{{Label: "总用量", Value: "1,000,000.00 M"}, {Label: "命中率", Value: "99.9%"}, {Label: "最长连烧", Value: "1,000 天"}},
		{{Label: "当前连烧", Value: "1 天"}, {Label: "请求", Value: "9,223,372"}, {Label: "用户回合", Value: "8"}},
	}
	got := KPIBox(cells, BoxUnicode)
	lines := strings.Split(strings.TrimRight(got, "\n"), "\n")
	w := DisplayWidth(lines[0])
	for i, line := range lines {
		if DisplayWidth(line) != w {
			t.Fatalf("line %d width %d want %d\n%q\n%s", i, DisplayWidth(line), w, line, got)
		}
	}
	if !strings.Contains(got, "1,000,000.00 M") {
		t.Fatalf("%s", got)
	}
}
