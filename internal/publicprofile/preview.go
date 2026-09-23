package publicprofile

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"strings"
	"time"
)

const (
	previewW     = 800
	previewH     = 194
	wallX        = 28.0
	wallY        = 88.0
	wallCell     = 13.0
	wallGap      = 1.5
	monthTickGap = 4.0
)

type Theme struct {
	Name      string
	Text      string
	Secondary string
	Empty     string
	Future    string
	Unknown   string
	Hairline  string
	Heat      HeatRamp
	Newsprint bool
}

var ThemeLightGreen = Theme{
	Name: "A · Emerald green",
	Text: "#1F2328", Secondary: "#59636E", Empty: "#EFF2F5",
	Future: "#F6F8FA", Unknown: "#D1D9E0", Hairline: "#D1D9E0",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.90, C: 0.090, H: 140},
		Mid:  OKLCH{L: 0.72, C: 0.130, H: 140},
		High: OKLCH{L: 0.54, C: 0.140, H: 140},
		Max:  OKLCH{L: 0.46, C: 0.147, H: 140},
	},
}

var ThemeLightCobalt = Theme{
	Name: "B · Cobalt blue",
	Text: "#1F2328", Secondary: "#59636E", Empty: "#EFF2F5",
	Future: "#F6F8FA", Unknown: "#D1D9E0", Hairline: "#D1D9E0",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.88, C: 0.055, H: 260},
		Mid:  OKLCH{L: 0.72, C: 0.140, H: 260},
		High: OKLCH{L: 0.54, C: 0.170, H: 260},
		Max:  OKLCH{L: 0.46, C: 0.180, H: 260},
	},
}

var ThemeLightIndigo = Theme{
	Name: "C · Electric indigo",
	Text: "#1F2328", Secondary: "#59636E", Empty: "#EFF2F5",
	Future: "#F6F8FA", Unknown: "#D1D9E0", Hairline: "#D1D9E0",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.88, C: 0.060, H: 290},
		Mid:  OKLCH{L: 0.72, C: 0.150, H: 290},
		High: OKLCH{L: 0.54, C: 0.210, H: 290},
		Max:  OKLCH{L: 0.46, C: 0.220, H: 290},
	},
}

var ThemeLightMagenta = Theme{
	Name: "D · Magenta red",
	Text: "#1F2328", Secondary: "#59636E", Empty: "#EFF2F5",
	Future: "#F6F8FA", Unknown: "#D1D9E0", Hairline: "#D1D9E0",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.88, C: 0.080, H: 340},
		Mid:  OKLCH{L: 0.72, C: 0.180, H: 340},
		High: OKLCH{L: 0.54, C: 0.180, H: 340},
		Max:  OKLCH{L: 0.46, C: 0.190, H: 340},
	},
}

var ThemeLightNewsprint = Theme{
	Name: "Newsprint",
	Text: "#1F2328", Secondary: "#59636E", Empty: "#EFF2F5",
	Future: "#F6F8FA", Unknown: "#D1D9E0", Hairline: "#D1D9E0",
	Newsprint: true,
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.86, C: 0, H: 0},
		Mid:  OKLCH{L: 0.67, C: 0, H: 0},
		High: OKLCH{L: 0.43, C: 0, H: 0},
		Max:  OKLCH{L: 0.20, C: 0, H: 0},
	},
}

// ThemeLight is the newsprint sheet. Bundle picks a palette-specific pair.
var ThemeLight = ThemeLightNewsprint

// Dark previews are drawn for a dark README, not produced by inverting the
// light bitmap. Newsprint activity gets lighter as the day gets larger.
var ThemeDarkNewsprint = Theme{
	Name: "Newsprint dark",
	Text: "#F2EEE8", Secondary: "#B7B0A8", Empty: "#24211E",
	Future: "#1C1916", Unknown: "#3A342E", Hairline: "#4A433C",
	Newsprint: true,
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.40, C: 0, H: 0},
		Mid:  OKLCH{L: 0.55, C: 0, H: 0},
		High: OKLCH{L: 0.72, C: 0, H: 0},
		Max:  OKLCH{L: 0.88, C: 0, H: 0},
	},
}

var ThemeDarkCobalt = Theme{
	Name: "Cobalt dark",
	Text: "#F0F3F6", Secondary: "#9EA7B3", Empty: "#24292F",
	Future: "#1C2128", Unknown: "#444C56", Hairline: "#373E47",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.42, C: 0.055, H: 260},
		Mid:  OKLCH{L: 0.56, C: 0.110, H: 260},
		High: OKLCH{L: 0.70, C: 0.130, H: 260},
		Max:  OKLCH{L: 0.82, C: 0.110, H: 260},
	},
}

var ThemeDarkMagenta = Theme{
	Name: "Magenta dark",
	Text: "#F0F3F6", Secondary: "#9EA7B3", Empty: "#24292F",
	Future: "#1C2128", Unknown: "#444C56", Hairline: "#373E47",
	Heat: HeatRamp{
		Low:  OKLCH{L: 0.42, C: 0.070, H: 340},
		Mid:  OKLCH{L: 0.56, C: 0.130, H: 340},
		High: OKLCH{L: 0.70, C: 0.140, H: 340},
		Max:  OKLCH{L: 0.82, C: 0.120, H: 340},
	},
}

var ThemeDark = ThemeDarkNewsprint

func ThemesFor(palette string) (light, dark Theme) {
	switch palette {
	case PaletteCobalt:
		return ThemeLightCobalt, ThemeDarkCobalt
	case PaletteMagenta:
		return ThemeLightMagenta, ThemeDarkMagenta
	default:
		return ThemeLightNewsprint, ThemeDarkNewsprint
	}
}

type monthTick struct {
	label string
	x     float64
	width float64
}

type weekdayTick struct {
	label string
	x     float64
	y     float64
	width float64
}

func RenderPreview(w io.Writer, s Snapshot, th Theme) error {
	var b strings.Builder
	writePreview(&b, s, th)
	_, err := io.WriteString(w, b.String())
	return err
}

func RenderRampStrip(w io.Writer, th Theme) error {
	var b strings.Builder
	b.WriteString(`<?xml version="1.0" encoding="UTF-8"?>` + "\n")
	b.WriteString(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 560 58" width="560" height="58" role="img">` + "\n")
	fmt.Fprintf(&b, `<title>%s</title>`+"\n", esc(th.Name+" continuous activity ramp"))
	labels := []string{"empty", "low", "medium", "high", "max"}
	fills := []string{th.Empty, heatColor(th.Heat, 0), heatColor(th.Heat, heatMidPosition), heatColor(th.Heat, heatHighPosition), heatColor(th.Heat, 1)}
	for i, fill := range fills {
		x := 4 + i*111
		fmt.Fprintf(&b, `<rect class="ramp-step" x="%d" y="4" width="103" height="28" rx="4" fill="%s"/>`+"\n", x, fill)
		text(&b, float64(x), 48, 10, th.Secondary, "start", "500", labels[i])
	}
	b.WriteString("</svg>\n")
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

	text(b, 4, 19, 13, th.Text, "start", "600", "whereToken")
	text(b, 86, 19, 12, th.Secondary, "start", "400", "my coding-agent token usage")
	updated := "updated " + previewDate(s.AsOfDate)
	if s.Provenance.Kind == ProvenanceSyntheticDemo {
		updated = "DEMO DATA · " + updated
	}
	text(b, 796, 19, 11, th.Secondary, "end", "400", updated)
	fmt.Fprintf(b, `<line x1="4" y1="31.5" x2="796" y2="31.5" stroke="%s" stroke-width="1"/>`+"\n", th.Hairline)

	text(b, 4, 65, 30, th.Text, "start", "600", all.Totals.Total.Display+" tokens")
	for _, tick := range monthTicks(s.Activity.Dates) {
		textClass(b, "month", tick.x, 82, 8, th.Secondary, "start", "500", tick.label)
	}
	for _, tick := range weekdayTicks() {
		textClass(b, "weekday", tick.x, tick.y, 8, th.Secondary, "end", "500", tick.label)
	}

	writeNewsprintDefs(b, s, th)
	writeWall(b, s, th, wallX, wallY)
	b.WriteString("</svg>\n")
}

func writeWall(b *strings.Builder, s Snapshot, th Theme, x, y float64) {
	ser := tokenSeries(s)
	scale := newMagnitudeScale(ser.Values, ser.States)
	for i := 0; i < len(s.Activity.Dates) && i < WallDays; i++ {
		col := i / 7
		row := i % 7
		cx := x + float64(col)*(wallCell+wallGap)
		cy := y + float64(row)*(wallCell+wallGap)
		fill := th.Empty
		state := CellEmpty
		if i < len(ser.States) {
			state = ser.States[i]
		}
		switch state {
		case CellFuture:
			fill = th.Future
		case CellUnknown:
			fill = th.Unknown
		case CellActive:
			if i < len(ser.Values) && ser.Values[i] > 0 {
				fill = heatColor(th.Heat, scale.intensity(ser.Values[i]))
				if th.Newsprint {
					fill = fmt.Sprintf("url(#newsprint-ink-%d)", i)
				}
			}
		}
		fmt.Fprintf(b, `<rect class="day" x="%.1f" y="%.1f" width="%.1f" height="%.1f" rx="1.5" fill="%s"/>`+"\n", cx, cy, wallCell, wallCell, fill)
	}
}

func writeNewsprintDefs(b *strings.Builder, s Snapshot, th Theme) {
	if !th.Newsprint {
		return
	}
	ser := tokenSeries(s)
	scale := newMagnitudeScale(ser.Values, ser.States)
	var defs strings.Builder
	for i := 0; i < len(s.Activity.Dates) && i < WallDays; i++ {
		if i >= len(ser.States) || ser.States[i] != CellActive || i >= len(ser.Values) || ser.Values[i] <= 0 {
			continue
		}
		ink, speck := newsprintInk(th.Heat, scale.intensity(ser.Values[i]), i)
		sx := (i * 3) % 11
		sy := (i * 5) % 11
		fmt.Fprintf(&defs, `<pattern id="newsprint-ink-%d" width="13" height="13" patternUnits="userSpaceOnUse"><rect width="13" height="13" fill="%s"/><rect x="%d" y="%d" width="1" height="1" fill="%s" fill-opacity="0.16"/></pattern>`+"\n", i, ink, sx, sy, speck)
	}
	if defs.Len() == 0 {
		return
	}
	b.WriteString("<defs>\n")
	b.WriteString(defs.String())
	b.WriteString("</defs>\n")
}

// PreviewMatchesPalette reports whether a rendered preview is the requested
// palette. Newsprint with activity uses newsprint-ink patterns. Any other
// palette must not. An empty newsprint sheet has no active cells and
// therefore no ink patterns.
func PreviewMatchesPalette(svg []byte, palette string, active bool) bool {
	if err := ValidatePalette(palette); err != nil {
		return false
	}
	ink := bytes.Contains(svg, []byte("newsprint-ink-"))
	if palette == PaletteNewsprint {
		if active {
			return ink
		}
		return !ink
	}
	return !ink
}

func SnapshotHasActiveDays(s Snapshot) bool {
	ser := tokenSeries(s)
	for i, state := range ser.States {
		if state == CellActive && i < len(ser.Values) && ser.Values[i] > 0 {
			return true
		}
	}
	return false
}

// newsprintInk shifts lightness by a fixed formation of the cell index.
// The shift is an art-direction stand-in for uneven pigment on the same
// sheet. It is not a calibrated absorption length.
func newsprintInk(ramp HeatRamp, intensity float64, cell int) (ink, speck string) {
	base := gamutMapToSRGB(ramp.colorAt(intensity))
	varied := base
	varied.L += inkJitter(cell) * 0.012
	if varied.L < 0 {
		varied.L = 0
	}
	if varied.L > 1 {
		varied.L = 1
	}
	fiber := varied
	fiber.L += 0.035
	if fiber.L > 1 {
		fiber.L = 1
	}
	return oklchHex(gamutMapToSRGB(varied)), oklchHex(gamutMapToSRGB(fiber))
}

func inkJitter(i int) float64 {
	n := uint32(i*1103515245 + 12345)
	return float64(n%2001)/1000 - 1
}

func tokenSeries(s Snapshot) Series {
	for _, item := range s.Activity.Series {
		if item.Dimension == "all" && (item.Metric == MetricTokens || item.Metric == "") {
			return item
		}
	}
	return Series{}
}

func monthTicks(dates []string) []monthTick {
	ticks := make([]monthTick, 0, 13)
	previousMonth := ""
	lastEnd := -monthTickGap
	for col := 0; col < 53 && col*7 < len(dates); col++ {
		date, err := time.Parse("2006-01-02", dates[col*7])
		if err != nil {
			continue
		}
		month := date.Format("Jan")
		if month == previousMonth {
			continue
		}
		previousMonth = month
		x := wallX + float64(col)*(wallCell+wallGap)
		width := float64(len(month)) * 4.8
		if x < lastEnd+monthTickGap || x+width > previewW {
			continue
		}
		ticks = append(ticks, monthTick{label: month, x: x, width: width})
		lastEnd = x + width
	}
	return ticks
}

func weekdayTicks() []weekdayTick {
	return []weekdayTick{
		{label: "Mon", x: 22, y: wallY + 10, width: 14.4},
		{label: "Wed", x: 22, y: wallY + 2*(wallCell+wallGap) + 10, width: 14.4},
		{label: "Fri", x: 22, y: wallY + 4*(wallCell+wallGap) + 10, width: 9.6},
	}
}

func previewDesc(s Snapshot) string {
	return "Coding-agent token usage through " + s.AsOfDate + ". No prompts, paths, or identifiers."
}

func previewDate(iso string) string {
	date, err := time.Parse("2006-01-02", iso)
	if err != nil {
		return iso
	}
	return date.Format("Jan 2")
}

func text(b *strings.Builder, x, y, size float64, fill, anchor, weight, value string) {
	textClass(b, "", x, y, size, fill, anchor, weight, value)
}

func textClass(b *strings.Builder, class string, x, y, size float64, fill, anchor, weight, value string) {
	classAttr := ""
	if class != "" {
		classAttr = ` class="` + esc(class) + `"`
	}
	fmt.Fprintf(b, `<text%s x="%.1f" y="%.1f" fill="%s" font-family="-apple-system, BlinkMacSystemFont, 'Segoe UI', Helvetica, Arial, sans-serif" font-size="%.1f" font-weight="%s" text-anchor="%s">%s</text>`+"\n",
		classAttr, x, y, fill, size, weight, anchor, esc(value))
}

func esc(value string) string {
	var b strings.Builder
	if err := xml.EscapeText(&b, []byte(value)); err != nil {
		return ""
	}
	return b.String()
}
