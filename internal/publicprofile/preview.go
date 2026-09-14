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
	Heat      [6]string
	Peak      string
}

var ThemeLight = Theme{
	Name: "light", Bg: "#F8FAFC", Surface: "#FFFFFF", Border: "#E2E8F0",
	Text: "#0F172A", Secondary: "#64748B", Empty: "#EEF2F6", Accent: "#4F46E5",
	Heat: [6]string{"#EEF2F6", "#E0E7FF", "#C7D2FE", "#A5B4FC", "#818CF8", "#4F46E5"},
	Peak: "#06B6D4",
}

var ThemeDark = Theme{
	Name: "dark", Bg: "#090B11", Surface: "#10141D", Border: "#252C3A",
	Text: "#F8FAFC", Secondary: "#94A3B8", Empty: "#1B2230", Accent: "#8B5CF6",
	Heat: [6]string{"#1B2230", "#2E1064", "#4C1D95", "#6D28D9", "#8B5CF6", "#C4B5FD"},
	Peak: "#22D3EE",
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
	fmt.Fprintf(b, `<metadata id="wheretoken-snapshot-id">%s</metadata>`+"\n", esc(s.SnapshotID))
	fmt.Fprintf(b, `<rect x="0.5" y="0.5" width="799" height="247" rx="12" fill="%s" stroke="%s" stroke-width="1"/>`+"\n", th.Bg, th.Border)
	text(b, 20, 28, 13, th.Text, "start", "600", "whereToken")
	text(b, 112, 28, 12, th.Secondary, "start", "400", "AI coding profile")
	text(b, 780, 28, 11, th.Secondary, "end", "400", "updated "+s.AsOfDate)

	total := all.Totals.Total.Display
	text(b, 20, 64, 28, th.Accent, "start", "700", total)
	text(b, 20, 82, 11, th.Secondary, "start", "400", "tokens tracked")

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
	footer := "Local public snapshot"
	if s.Provenance.Kind == ProvenanceSyntheticDemo {
		footer = "DEMO DATA · synthetic snapshot"
	}
	text(b, 20, 232, 11, th.Secondary, "start", "400", footer)
	text(b, 780, 232, 11, th.Accent, "end", "600", "Explore profile →")
	b.WriteString("</svg>\n")
}

func writeWall(b *strings.Builder, s Snapshot, th Theme, x, y float64) {
	ser := Series{}
	for _, it := range s.Activity.Series {
		if it.Dimension == "all" && (it.Metric == MetricTokens || it.Metric == "") {
			ser = it
			break
		}
	}
	cell, gap := 9.0, 2.0
	peakIdx := -1
	peakVal := int64(-1)
	for i := 0; i < len(ser.Values) && i < WallDays; i++ {
		if i < len(ser.States) && ser.States[i] == CellActive && ser.Values[i] > peakVal {
			peakVal = ser.Values[i]
			peakIdx = i
		}
	}
	for i := 0; i < len(s.Activity.Dates) && i < WallDays; i++ {
		col := i / 7
		row := i % 7
		cx := x + float64(col)*(cell+gap)
		cy := y + float64(row)*(cell+gap)
		fill := th.Empty
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
			lv := 1
			if i < len(ser.Levels) && ser.Levels[i] > 0 {
				lv = ser.Levels[i]
			}
			if lv > 5 {
				lv = 5
			}
			fill = th.Heat[lv]
		}
		stroke := ""
		if i == peakIdx && st == CellActive {
			stroke = fmt.Sprintf(` stroke="%s" stroke-width="1"`, th.Peak)
		}
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="1.5" fill="%s"%s/>`+"\n", cx, cy, cell, cell, fill, stroke)
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
