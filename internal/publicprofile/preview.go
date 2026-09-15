package publicprofile

import (
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	previewW = 800
	previewH = 204
)

type Theme struct {
	Name      string
	Bg        string
	Surface   string
	Border    string
	Text      string
	Secondary string
	Empty     string
	Heat      [6]string
}

var ThemeLight = Theme{
	Name: "light", Bg: "#F7F6F3", Surface: "#FBFAF8", Border: "#DEDAD4",
	Text: "#201D19", Secondary: "#756D64", Empty: "#EEEAE4",
	Heat: [6]string{"#EEEAE4", "#E4D4C1", "#D5AE86", "#C57E4A", "#A65331", "#71321F"},
}

var ThemeDark = Theme{
	Name: "dark", Bg: "#11100F", Surface: "#171513", Border: "#2A2724",
	Text: "#F2EEE8", Secondary: "#A79D92", Empty: "#24211E",
	Heat: [6]string{"#24211E", "#3A2B24", "#65402D", "#965837", "#C77845", "#F1A260"},
}

func RenderPreview(w io.Writer, s Snapshot, th Theme) error {
	var b strings.Builder
	writePreview(&b, s, th)
	_, err := io.WriteString(w, b.String())
	return err
}

func writePreview(b *strings.Builder, s Snapshot, th Theme) {
	all := s.Periods.All
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	fmt.Fprintf(b, `<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 %d %d" width="%d" height="%d" role="img">`+"\n", previewW, previewH, previewW, previewH)
	fmt.Fprintf(b, `<title>%s</title>`+"\n", esc("whereToken public coding-agent usage preview"))
	fmt.Fprintf(b, `<desc>%s</desc>`+"\n", esc(previewDesc(s)))
	fmt.Fprintf(b, `<metadata id="wheretoken-snapshot-id">%s</metadata>`+"\n", esc(s.SnapshotID))
	fmt.Fprintf(b, `<rect x="0.5" y="0.5" width="799" height="203" rx="6" fill="%s" stroke="%s" stroke-width="1"/>`+"\n", th.Bg, th.Border)
	text(b, 30, 27, 13, th.Text, "start", "600", "whereToken")
	text(b, 112, 27, 12, th.Secondary, "start", "400", "my coding-agent token usage")
	updated := "updated " + previewDate(s.AsOfDate)
	if s.Provenance.Kind == ProvenanceSyntheticDemo {
		updated = "DEMO DATA · " + updated
	}
	text(b, 770, 27, 11, th.Secondary, "end", "400", updated)
	fmt.Fprintf(b, `<line x1="30" y1="43.5" x2="770" y2="43.5" stroke="%s" stroke-width="1"/>`+"\n", th.Border)

	total := all.Totals.Total.Display
	text(b, 30, 79, 30, th.Text, "start", "600", total+" tokens")
	text(b, 770, 82, 10, th.Secondary, "end", "500", "53 weeks")

	writeWall(b, s, th, 30, 96)
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
	cell, gap := 12.0, 2.0
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
		fmt.Fprintf(b, `<rect x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="1.5" fill="%s"/>`+"\n", cx, cy, cell, cell, fill)
	}
}

func previewDesc(s Snapshot) string {
	return "Coding-agent token usage through " + s.AsOfDate + ". No prompts, paths, or identifiers."
}

func previewDate(iso string) string {
	d, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return d.Format("Jan 2")
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
