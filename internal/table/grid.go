package table

import "strings"

type Align int

const (
	AlignLeft Align = iota
	AlignRight
)

type BoxStyle struct {
	TL, TR, BL, BR, H, V, TJ, BJ, LJ, RJ, X, Ellipsis string
}

var BoxUnicode = BoxStyle{
	TL: "┌", TR: "┐", BL: "└", BR: "┘",
	H: "─", V: "│",
	TJ: "┬", BJ: "┴", LJ: "├", RJ: "┤", X: "┼",
	Ellipsis: "…",
}

var BoxASCII = BoxStyle{
	TL: "+", TR: "+", BL: "+", BR: "+",
	H: "-", V: "|",
	TJ: "+", BJ: "+", LJ: "+", RJ: "+", X: "+",
	Ellipsis: "...",
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
	cellLeft := func(text string, w int) string {
		return " " + PadRight(text, w-2) + " "
	}
	cellRight := func(text string, w int) string {
		return " " + PadLeft(text, w-2) + " "
	}
	hline := func(left, mid, right string) string {
		return left + strings.Repeat(style.H, colW[0]) + mid + strings.Repeat(style.H, colW[1]) + mid + strings.Repeat(style.H, colW[2]) + right
	}
	rowLeft := func(a, b, c string) string {
		return style.V + cellLeft(a, colW[0]) + style.V + cellLeft(b, colW[1]) + style.V + cellLeft(c, colW[2]) + style.V
	}
	rowRight := func(a, b, c string) string {
		return style.V + cellRight(a, colW[0]) + style.V + cellRight(b, colW[1]) + style.V + cellRight(c, colW[2]) + style.V
	}
	var b strings.Builder
	b.WriteString(hline(style.TL, style.TJ, style.TR))
	b.WriteByte('\n')
	b.WriteString(rowLeft(cells[0][0].Label, cells[0][1].Label, cells[0][2].Label))
	b.WriteByte('\n')
	b.WriteString(rowRight(cells[0][0].Value, cells[0][1].Value, cells[0][2].Value))
	b.WriteByte('\n')
	b.WriteString(hline(style.LJ, style.X, style.RJ))
	b.WriteByte('\n')
	b.WriteString(rowLeft(cells[1][0].Label, cells[1][1].Label, cells[1][2].Label))
	b.WriteByte('\n')
	b.WriteString(rowRight(cells[1][0].Value, cells[1][1].Value, cells[1][2].Value))
	b.WriteByte('\n')
	b.WriteString(hline(style.BL, style.BJ, style.BR))
	b.WriteByte('\n')
	return b.String()
}

func RankedTable(headers []string, rows [][]string, align []Align, style BoxStyle) string {
	return FitRankedTable(headers, rows, align, style, 0)
}

func FitRankedTable(headers []string, rows [][]string, align []Align, style BoxStyle, maxWidth int) string {
	headers = append([]string(nil), headers...)
	align = append([]Align(nil), align...)
	cloned := make([][]string, len(rows))
	for i, row := range rows {
		cloned[i] = append([]string(nil), row...)
	}
	rows = cloned

	widthsFor := func(hs []string, rs [][]string) []int {
		n := len(hs)
		widths := make([]int, n)
		for i, h := range hs {
			widths[i] = DisplayWidth(h)
		}
		for _, row := range rs {
			for i := 0; i < n && i < len(row); i++ {
				widths[i] = maxInt(widths[i], DisplayWidth(row[i]))
			}
		}
		return widths
	}
	tableWidth := func(widths []int) int {
		if len(widths) == 0 {
			return 0
		}
		const sep = 3
		total := sep * (len(widths) - 1)
		for _, w := range widths {
			total += w
		}
		return total
	}
	trimTo := func(n int) {
		if n < 0 {
			n = 0
		}
		if n >= len(headers) {
			return
		}
		headers = headers[:n]
		if len(align) > n {
			align = align[:n]
		}
		for i := range rows {
			if len(rows[i]) > n {
				rows[i] = rows[i][:n]
			}
		}
	}

	widths := widthsFor(headers, rows)
	if maxWidth > 0 {
		for len(headers) > 2 && tableWidth(widths) > maxWidth {
			trimTo(len(headers) - 1)
			widths = widthsFor(headers, rows)
		}
		if len(headers) > 0 && tableWidth(widths) > maxWidth {
			shrink := tableWidth(widths) - maxWidth
			min0 := DisplayWidth(headers[0])
			if min0 < 4 {
				min0 = 4
			}
			if widths[0]-shrink < min0 {
				widths[0] = min0
			} else {
				widths[0] -= shrink
			}
		}
	}
	cols := len(headers)
	ell := style.Ellipsis
	if ell == "" {
		ell = "…"
	}
	clip := func(s string, i int) string {
		if i < len(widths) && DisplayWidth(s) > widths[i] {
			return TruncateEllipsis(s, widths[i], ell)
		}
		return s
	}
	pad := func(s string, i int) string {
		if i < len(align) && align[i] == AlignRight {
			return PadLeft(s, widths[i])
		}
		return PadRight(s, widths[i])
	}
	join := func(cells []string) string {
		return strings.Join(cells, "   ")
	}
	head := make([]string, cols)
	for i, h := range headers {
		head[i] = pad(clip(h, i), i)
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
				s = clip(row[i], i)
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
