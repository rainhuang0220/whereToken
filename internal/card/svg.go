package card

import (
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"strings"
)

const (
	svgWidth  = 800
	svgHeight = 576
	fontStack = "ui-monospace, SFMono-Regular, Menlo, Consolas, 'Liberation Mono', monospace"

	colBg        = "#0b0f10"
	colBorder    = "#1e262a"
	colText      = "#f4f7f7"
	colSecondary = "#8b989a"
	colMuted     = "#55626a"
	colPurple    = "#8787af"
	colAmber     = "#ff8700"
	colEmpty     = "#141a1d"
	colTrack     = "#161c1f"

	cellSize = 11.0
	cellGap  = 2.0
	wallX    = 56.0
	wallY    = 278.0
)

var levelOpacity = [5]float64{0, 0.28, 0.50, 0.72, 1.00}

// Render writes a pure, deterministic SVG for c. It accepts only PublicCard.
func Render(w io.Writer, c PublicCard) error {
	var b strings.Builder
	writeSVG(&b, c)
	_, err := io.WriteString(w, b.String())
	return err
}

func writeSVG(b *strings.Builder, c PublicCard) {
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>`)
	b.WriteByte('\n')
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img">`, svgWidth, svgHeight, svgWidth, svgHeight)
	b.WriteByte('\n')
	fmt.Fprintf(b, `<title>%s</title>`, esc(svgTitle(c)))
	b.WriteByte('\n')
	fmt.Fprintf(b, `<desc>%s</desc>`, esc(svgDesc(c)))
	b.WriteByte('\n')
	b.WriteString(`<defs>`)
	b.WriteByte('\n')
	b.WriteString(`<pattern id="wt-scanline" width="800" height="4" patternUnits="userSpaceOnUse"><rect width="800" height="1" fill="#000000" fill-opacity="0.06"/></pattern>`)
	b.WriteByte('\n')
	b.WriteString(`<pattern id="wt-unknown" width="4" height="4" patternUnits="userSpaceOnUse"><rect width="4" height="4" fill="#141a1d"/><path d="M-1 1l2-2M0 4l4-4M3 5l2-2" stroke="#55626a" stroke-width="0.8" fill="none"/></pattern>`)
	b.WriteByte('\n')
	b.WriteString(`</defs>`)
	b.WriteByte('\n')
	fmt.Fprintf(b, `<rect x="1" y="1" width="798" height="574" fill="%s" stroke="%s" stroke-width="1"/>`, colBg, colBorder)
	b.WriteByte('\n')

	writeMark(b)
	text(b, 52, 48, 20, colText, "start", "700", "whereToken")
	text(b, 772, 48, 12, colSecondary, "end", "400", versionLabel(c.Version))

	text(b, 32, 126, 52, colAmber, "start", "700", c.AllTimeTokens.Display)
	textLS(b, 32, 148, 10, colSecondary, "start", "TOKENS BURNED · ALL TIME")
	text(b, 520, 118, 28, colText, "start", "600", c.Last53WeeksTokens.Display)
	textLS(b, 520, 148, 10, colSecondary, "start", "LAST 53 WEEKS")
	if c.DataStatus == DataUnavailable {
		textLS(b, 32, 64, 10, colMuted, "start", "NO LOCAL USAGE AVAILABLE")
	}

	fmt.Fprintf(b, `<line x1="24" y1="168" x2="776" y2="168" stroke="%s" stroke-width="1"/>`, colBorder)
	b.WriteByte('\n')

	writeMetric(b, 32, "CURRENT STREAK", streakDisplay(c.CurrentStreakDays, c.DataStatus))
	writeMetric(b, 210, "LONGEST STREAK", streakDisplay(c.LongestStreakDays, c.DataStatus))
	writeMetric(b, 388, "ACTIVE DAYS · 53W", streakDisplay(c.ActiveDays53Weeks, c.DataStatus))
	writeCostMetric(b, 566, c.Cost)

	textLS(b, 32, 264, 12, colSecondary, "start", "VIBE CODING WALL")
	text(b, 772, 264, 11, colSecondary, "end", "400", peakCaption(c.Peak))

	writeWall(b, c)

	textLS(b, 32, 401, 12, colSecondary, "start", "WHERE TOKENS WENT")
	writeAgents(b, c.Agents)

	fmt.Fprintf(b, `<line x1="24" y1="534" x2="776" y2="534" stroke="%s" stroke-width="1"/>`, colBorder)
	b.WriteByte('\n')
	text(b, 32, 556, 11, colMuted, "start", "400", "$ wheretoken card profile.svg")
	text(b, 772, 556, 11, colMuted, "end", "400", "github.com/rainhuang0220/whereToken")

	b.WriteString(`<rect x="1" y="1" width="798" height="574" fill="url(#wt-scanline)" pointer-events="none"/>`)
	b.WriteByte('\n')
	b.WriteString(`</svg>`)
	b.WriteByte('\n')
}

func writeMark(b *strings.Builder) {
	fmt.Fprintf(b, `<rect x="24" y="38" width="18" height="8" rx="1.2" fill="%s"/>`, colAmber)
	b.WriteByte('\n')
	fmt.Fprintf(b, `<rect x="27" y="34" width="18" height="8" rx="1.2" fill="%s" fill-opacity="0.85"/>`, colPurple)
	b.WriteByte('\n')
	fmt.Fprintf(b, `<rect x="30" y="30" width="18" height="8" rx="1.2" fill="%s" fill-opacity="0.65"/>`, colAmber)
	b.WriteByte('\n')
}

func writeMetric(b *strings.Builder, x float64, label, value string) {
	textLS(b, x, 198, 9, colMuted, "start", label)
	text(b, x, 224, 20, colText, "start", "600", value)
}

func writeCostMetric(b *strings.Builder, x float64, cost Cost) {
	textLS(b, x, 198, 9, colMuted, "start", "API LIST-PRICE EQUIV.")
	fmt.Fprintf(b, `<text x="%.1f" y="224.0" fill="%s" font-family="%s" font-size="20.0" font-weight="600" text-anchor="start">%s`,
		x, colText, fontStack, esc(cost.Display))
	if cost.Status == "partial" && cost.Display != emDash {
		fmt.Fprintf(b, `<tspan fill="%s" font-size="9.0" font-weight="400" dx="10" letter-spacing="1.2">PARTIAL</tspan>`, colAmber)
	}
	b.WriteString(`</text>`)
	b.WriteByte('\n')
}

func writeWall(b *strings.Builder, c PublicCard) {
	step := cellSize + cellGap
	peakDate := ""
	if c.Peak.Available && c.Peak.VisibleInWall {
		peakDate = c.Peak.Date
	}
	for i, cell := range c.Cells {
		col := i / 7
		row := i % 7
		x := wallX + float64(col)*step
		y := wallY + float64(row)*step
		switch cell.State {
		case CellFuture:
			fmt.Fprintf(b, `<rect data-date="%s" data-state="future" data-level="0" x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="%s" stroke-width="1"/>`, esc(cell.Date), x, y, cellSize, cellSize, colBorder)
		case CellUnknown:
			fmt.Fprintf(b, `<rect data-date="%s" data-state="unknown" data-level="0" x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="url(#wt-unknown)"/>`, esc(cell.Date), x, y, cellSize, cellSize)
		case CellActive:
			op := 1.0
			if cell.Level >= 0 && cell.Level < len(levelOpacity) {
				op = levelOpacity[cell.Level]
			}
			fmt.Fprintf(b, `<rect data-date="%s" data-state="active" data-level="%d" x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s" fill-opacity="%.2f"/>`, esc(cell.Date), cell.Level, x, y, cellSize, cellSize, colPurple, op)
		default:
			fmt.Fprintf(b, `<rect data-date="%s" data-state="empty" data-level="0" x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="%s"/>`, esc(cell.Date), x, y, cellSize, cellSize, colEmpty)
		}
		b.WriteByte('\n')
		if peakDate != "" && cell.Date == peakDate {
			fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" fill="none" stroke="%s" stroke-width="1"/>`, x, y, cellSize, cellSize, colAmber)
			b.WriteByte('\n')
		}
	}
}

func writeAgents(b *strings.Builder, agents []Agent) {
	ys := []float64{426, 450, 474, 498}
	const trackX = 220.0
	const trackW = 430.0
	for i, a := range agents {
		if i >= len(ys) {
			break
		}
		y := ys[i]
		text(b, 32, y, 12, colText, "start", "400", a.Label)
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="8" rx="1" fill="%s"/>`, trackX, y-8, trackW, colTrack)
		b.WriteByte('\n')
		bar := math.Round(a.ShareRatio * trackW)
		if bar < 0 {
			bar = 0
		}
		if bar > trackW {
			bar = trackW
		}
		if bar >= 1 {
			fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="8" rx="1" fill="%s"/>`, trackX, y-8, bar, colPurple)
			b.WriteByte('\n')
		}
		text(b, 772, y, 12, colSecondary, "end", "400", a.ShareText)
	}
}

func svgTitle(c PublicCard) string {
	return "whereToken Vibe Coding Wall"
}

func svgDesc(c PublicCard) string {
	var b strings.Builder
	b.WriteString("Local coding-agent token usage as of ")
	b.WriteString(c.AsOfDate)
	b.WriteString(". All-time ")
	b.WriteString(c.AllTimeTokens.Display)
	b.WriteString(" tokens, last 53 weeks ")
	b.WriteString(c.Last53WeeksTokens.Display)
	b.WriteString(". Current streak ")
	b.WriteString(streakDisplay(c.CurrentStreakDays, c.DataStatus))
	b.WriteString(". Generated from this machine's ledgers; no prompts, paths, or identifiers.")
	return b.String()
}

func peakCaption(p Peak) string {
	if !p.Available {
		return "PEAK —"
	}
	return "PEAK " + p.Date + " · " + p.TokensDisplay
}

func streakDisplay(n int, status string) string {
	if status == DataUnavailable {
		return emDash
	}
	return strconvI(n)
}

func versionLabel(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return "dev"
	}
	if !(strings.HasPrefix(v, "v") || strings.HasPrefix(v, "V")) && v[0] >= '0' && v[0] <= '9' {
		v = "v" + v
	}
	// go build pseudo-versions like v0.7.1-0.20260906060051-e0cfcd35b64d
	// overflow the header; keep a short build label.
	if i := strings.Index(v, "-0."); i > 0 {
		v = v[:i] + "-dev"
	}
	return v
}

func text(b *strings.Builder, x, y, size float64, fill, anchor, weight, s string) {
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="%s" font-family="%s" font-size="%.1f" font-weight="%s" text-anchor="%s">%s</text>`,
		x, y, fill, fontStack, size, weight, anchor, esc(s))
	b.WriteByte('\n')
}

func textLS(b *strings.Builder, x, y, size float64, fill, anchor, s string) {
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="%s" font-family="%s" font-size="%.1f" font-weight="400" text-anchor="%s" letter-spacing="1.2">%s</text>`,
		x, y, fill, fontStack, size, anchor, esc(s))
	b.WriteByte('\n')
}

func esc(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return ""
	}
	return b.String()
}

func strconvI(n int) string {
	return fmt.Sprintf("%d", n)
}
