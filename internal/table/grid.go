package table

import "strings"

type Align int

const (
	AlignLeft Align = iota
	AlignRight
)

type BoxStyle struct {
	TL, TR, BL, BR, H, V, TJ, BJ, LJ, RJ, X string
}

var BoxUnicode = BoxStyle{
	TL: "┌", TR: "┐", BL: "└", BR: "┘",
	H: "─", V: "│",
	TJ: "┬", BJ: "┴", LJ: "├", RJ: "┤", X: "┼",
}

var BoxASCII = BoxStyle{
	TL: "+", TR: "+", BL: "+", BR: "+",
	H: "-", V: "|",
	TJ: "+", BJ: "+", LJ: "+", RJ: "+", X: "+",
}

type KPI struct {
	Label, Value string
}

func KPIBox(cells [2][3]KPI, style BoxStyle) string {
	colW := [3]int{}
	for c := 0; c < 3; c++ {
		for r := 0; r < 2; r++ {
			colW[c] = maxInt(colW[c], DisplayWidth(cells[r][c].Label))
			colW[c] = maxInt(colW[c], DisplayWidth(cells[r][c].Value))
		}
		colW[c] += 2
		if colW[c] < 12 {
			colW[c] = 12
		}
	}
	cell := func(text string, w int) string {
		return " " + PadRight(text, w-2) + " "
	}
	hline := func(left, mid, right string) string {
		return left + strings.Repeat(style.H, colW[0]) + mid + strings.Repeat(style.H, colW[1]) + mid + strings.Repeat(style.H, colW[2]) + right
	}
	row := func(a, b, c string) string {
		return style.V + cell(a, colW[0]) + style.V + cell(b, colW[1]) + style.V + cell(c, colW[2]) + style.V
	}
	var b strings.Builder
	b.WriteString(hline(style.TL, style.TJ, style.TR))
	b.WriteByte('\n')
	b.WriteString(row(cells[0][0].Label, cells[0][1].Label, cells[0][2].Label))
	b.WriteByte('\n')
	b.WriteString(row(cells[0][0].Value, cells[0][1].Value, cells[0][2].Value))
	b.WriteByte('\n')
	b.WriteString(hline(style.LJ, style.X, style.RJ))
	b.WriteByte('\n')
	b.WriteString(row(cells[1][0].Label, cells[1][1].Label, cells[1][2].Label))
	b.WriteByte('\n')
	b.WriteString(row(cells[1][0].Value, cells[1][1].Value, cells[1][2].Value))
	b.WriteByte('\n')
	b.WriteString(hline(style.BL, style.BJ, style.BR))
	b.WriteByte('\n')
	return b.String()
}

func RankedTable(headers []string, rows [][]string, align []Align, style BoxStyle) string {
	cols := len(headers)
	widths := make([]int, cols)
	for i, h := range headers {
		widths[i] = DisplayWidth(h)
	}
	for _, row := range rows {
		for i := 0; i < cols && i < len(row); i++ {
			widths[i] = maxInt(widths[i], DisplayWidth(row[i]))
		}
	}
	pad := func(s string, i int) string {
		if i < len(align) && align[i] == AlignRight {
			return PadLeft(s, widths[i])
		}
		return PadRight(s, widths[i])
	}
	join := func(cells []string) string {
		parts := make([]string, len(cells))
		copy(parts, cells)
		return strings.Join(parts, "   ")
	}
	head := make([]string, cols)
	for i, h := range headers {
		head[i] = pad(h, i)
	}
	var b strings.Builder
	top := join(head)
	b.WriteString(top)
	b.WriteByte('\n')
	b.WriteString(strings.Repeat(style.H, DisplayWidth(top)))
	b.WriteByte('\n')
	for _, row := range rows {
		cells := make([]string, cols)
		for i := 0; i < cols; i++ {
			s := ""
			if i < len(row) {
				s = row[i]
			}
			cells[i] = pad(s, i)
		}
		b.WriteString(join(cells))
		b.WriteByte('\n')
	}
	return b.String()
}

func maxInt(a, b int) int {
	if a > b {
		return a
	}
	return b
}
