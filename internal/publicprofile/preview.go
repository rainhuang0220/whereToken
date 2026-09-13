package publicprofile

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
)

const (
	previewW = 800
	previewH = 248
)

type Theme struct {
	Name      string
	Bg        string
	Surface   string
	Border    string
	Text      string
	Secondary string
	Empty     string
	Accent    string
	Heat      [5]float64
}

var ThemeLight = Theme{
	Name: "light", Bg: "#ffffff", Surface: "#f6f8fa", Border: "#d0d7de",
	Text: "#1f2328", Secondary: "#656d76", Empty: "#ebedf0", Accent: "#bf4a16",
	Heat: [5]float64{0, 0.22, 0.45, 0.7, 1},
}

var ThemeDark = Theme{
	Name: "dark", Bg: "#0d1117", Surface: "#161b22", Border: "#30363d",
	Text: "#e6edf3", Secondary: "#8b949e", Empty: "#21262d", Accent: "#e85d04",
	Heat: [5]float64{0, 0.28, 0.5, 0.72, 1},
}

func RenderPreview(w io.Writer, s Snapshot, th Theme) error {
	var b strings.Builder
	writePreview(&b, s, th)
	_, err := io.WriteString(w, b.String())
	return err
}

func writePreview(b *strings.Builder, s Snapshot, th Theme) {
	all := s.Periods.All
	w53 := s.Periods.W53
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img">`+"\n", previewW, previewH, previewW, previewH)
	fmt.Fprintf(b, `<title>%s</title>`+"\n", esc("whereToken public coding-agent usage preview"))
	fmt.Fprintf(b, `<desc>%s</desc>`+"\n", esc(previewDesc(s)))
	fmt.Fprintf(b, `<rect x="0.5" y="0.5" width="799" height="247" rx="8" fill="%s" stroke="%s" stroke-width="1"/>`+"\n", th.Bg, th.Border)
	text(b, 20, 28, 13, th.Text, "start", "600", "whereToken")
	text(b, 108, 28, 12, th.Secondary, "start", "400", "AI coding activity")
	text(b, 780, 28, 11, th.Secondary, "end", "400", "as of "+s.AsOfDate)

	total := all.Totals.Total.Display
	text(b, 20, 64, 28, th.Accent, "start", "700", total)
	text(b, 20, 82, 11, th.Secondary, "start", "400", "tokens burned · all time")

	streak := "—"
	if all.CurrentStreak.Display != "" {
		streak = all.CurrentStreak.Display
	}
	active := "—"
	if w53.ActiveDays.Display != "" {
		active = w53.ActiveDays.Display
	}
	text(b, 420, 58, 12, th.Text, "start", "600", streak+" day streak")
	text(b, 420, 76, 12, th.Text, "start", "600", active+" active days · 53w")
	top := "—"
	if len(all.ByAgent) > 0 {
		top = all.ByAgent[0].Label
	}
	text(b, 420, 94, 12, th.Secondary, "start", "400", "top "+top)

	writeWall(b, s, th, 20, 112)
	text(b, 20, 232, 11, th.Secondary, "start", "400", "Local public snapshot")
	text(b, 780, 232, 11, th.Accent, "end", "600", "View interactive profile →")
	b.WriteString("</svg>\n")
}

func writeWall(b *strings.Builder, s Snapshot, th Theme, x, y float64) {
	ser := Series{}
	for _, it := range s.Activity.Series {
		if it.Dimension == "all" {
			ser = it
			break
		}
	}
	cell, gap := 9.0, 2.0
	for i := 0; i < len(s.Activity.Dates) && i < WallDays; i++ {
		col := i / 7
		row := i % 7
		cx := x + float64(col)*(cell+gap)
		cy := y + float64(row)*(cell+gap)
		fill := th.Empty
		op := 1.0
		st := CellEmpty
		if i < len(ser.States) {
			st = ser.States[i]
		}
		switch st {
		case CellFuture:
			fill = th.Surface
		case CellUnknown:
			fill = th.Border
		case CellActive:
			fill = th.Accent
			lv := 0
			if i < len(ser.Levels) {
				lv = ser.Levels[i]
			}
			if lv >= 0 && lv < 5 {
				op = th.Heat[lv]
			}
		}
		if op < 1 {
			fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="1.5" fill="%s" fill-opacity="%.2f"/>`+"\n", cx, cy, cell, cell, fill, op)
		} else {
			fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="1.5" fill="%s"/>`+"\n", cx, cy, cell, cell, fill)
		}
	}
}

func previewDesc(s Snapshot) string {
	return "Public snapshot of local coding-agent token usage as of " + s.AsOfDate + ". No prompts, paths, or identifiers."
}

func text(b *strings.Builder, x, y, size float64, fill, anchor, weight, s string) {
	fmt.Fprintf(b, `<text x="%.1f" y="%.1f" fill="%s" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif" font-size="%.1f" font-weight="%s" text-anchor="%s">%s</text>`+"\n",
		x, y, fill, size, weight, anchor, esc(s))
}

func esc(s string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(s)); err != nil {
		return ""
	}
	return b.String()
}
