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

func KPIBox(rows [2][]KPI, style BoxStyle) string {
	return FitKPIBox(rows, style, 0)
}

func FitKPIBox(rows [2][]KPI, style BoxStyle, maxWidth int) string {
	cols := len(rows[0])
	if cols == 0 {
		return ""
	}
	if len(rows[1]) < cols {
		pad := make([]KPI, cols-len(rows[1]))
		rows[1] = append(append([]KPI(nil), rows[1]...), pad...)
	}
	clip := func(s string, inner int) string {
		if inner < 1 {
			inner = 1
		}
		if DisplayWidth(s) <= inner {
			return s
		}
		return TruncateEllipsis(s, inner, style.Ellipsis)
	}
	colW := make([]int, cols)
	for c := 0; c < cols; c++ {
		for r := 0; r < 2; r++ {
			if c >= len(rows[r]) {
				continue
			}
			colW[c] = maxInt(colW[c], DisplayWidth(rows[r][c].Label))
			colW[c] = maxInt(colW[c], DisplayWidth(rows[r][c].Value))
		}
		colW[c] += 2
		if colW[c] < 12 {
			colW[c] = 12
		}
	}
	if maxWidth > 0 {
		for {
			total := cols + 1
			for _, w := range colW {
				total += w
			}
			if total <= maxWidth {
				break
			}
			widest := 0
			for c := 1; c < cols; c++ {
				if colW[c] > colW[widest] {
					widest = c
				}
			}
			if colW[widest] <= 4 {
				break
			}
			colW[widest]--
		}
	}
	cellLeft := func(text string, w int) string {
		return " " + PadRight(clip(text, w-2), w-2) + " "
	}
	cellRight := func(text string, w int) string {
		return " " + PadLeft(clip(text, w-2), w-2) + " "
	}
	hline := func(left, mid, right string) string {
		var b strings.Builder
		b.WriteString(left)
		for i, w := range colW {
			b.WriteString(strings.Repeat(style.H, w))
			if i+1 < cols {
				b.WriteString(mid)
			}
		}
		b.WriteString(right)
		return b.String()
	}
	rowOf := func(vals []string, right bool) string {
		var b strings.Builder
		b.WriteString(style.V)
		for i := 0; i < cols; i++ {
			s := ""
			if i < len(vals) {
				s = vals[i]
			}
			if right {
				b.WriteString(cellRight(s, colW[i]))
			} else {
				b.WriteString(cellLeft(s, colW[i]))
			}
			b.WriteString(style.V)
		}
		return b.String()
	}
	labels := func(r int) []string {
		out := make([]string, cols)
		for c := 0; c < cols && c < len(rows[r]); c++ {
			out[c] = rows[r][c].Label
		}
		return out
	}
	values := func(r int) []string {
		out := make([]string, cols)
		for c := 0; c < cols && c < len(rows[r]); c++ {
			out[c] = rows[r][c].Value
		}
		return out
	}
	var b strings.Builder
	b.WriteString(hline(style.TL, style.TJ, style.TR))
	b.WriteByte('\n')
	b.WriteString(rowOf(labels(0), false))
	b.WriteByte('\n')
	b.WriteString(rowOf(values(0), true))
	b.WriteByte('\n')
	b.WriteString(hline(style.LJ, style.X, style.RJ))
	b.WriteByte('\n')
	b.WriteString(rowOf(labels(1), false))
	b.WriteByte('\n')
	b.WriteString(rowOf(values(1), true))
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
