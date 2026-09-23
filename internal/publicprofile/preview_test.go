package publicprofile

import (
	"bytes"
	"math"
	"regexp"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestPreviewMatchesRequestedPalette(t *testing.T) {
	loc := time.UTC
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, loc)
	snap, err := Build(Input{
		Now: now, Loc: loc, Version: "test",
		Events: []event.UsageEvent{{
			Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour),
			Miss: 100, Output: 20, Quality: event.QualityAuthoritative,
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !SnapshotHasActiveDays(snap) {
		t.Fatal("fixture has no active day")
	}
	var news, magenta bytes.Buffer
	if err := RenderPreview(&news, snap, ThemeLightNewsprint); err != nil {
		t.Fatal(err)
	}
	if err := RenderPreview(&magenta, snap, ThemeLightMagenta); err != nil {
		t.Fatal(err)
	}
	if !PreviewMatchesPalette(news.Bytes(), PaletteNewsprint, true) || !bytes.Contains(news.Bytes(), []byte("newsprint-ink-")) {
		t.Fatal("newsprint preview lost its ink")
	}
	if PreviewMatchesPalette(magenta.Bytes(), PaletteNewsprint, true) || bytes.Contains(magenta.Bytes(), []byte("newsprint-ink-")) {
		t.Fatal("magenta preview was accepted as newsprint")
	}
	if !PreviewMatchesPalette(magenta.Bytes(), PaletteMagenta, true) {
		t.Fatal("magenta preview was rejected")
	}
}

func TestMagnitudeScaleUsesFixedAbsoluteCapAcrossDatasets(t *testing.T) {
	active := func(n int) []string {
		states := make([]string, n)
		for i := range states {
			states[i] = CellActive
		}
		return states
	}
	isolated := newMagnitudeScale([]int64{200_000_000}, active(1))
	crowded := newMagnitudeScale([]int64{1, 10_000_000, 800_000_000, 10_000_000_000}, active(4))
	if got, want := isolated.intensity(200_000_000), math.Sqrt(0.2); math.Abs(got-want) > 1e-12 {
		t.Fatalf("isolated 200M intensity=%g want=%g", got, want)
	}
	if got, want := crowded.intensity(200_000_000), math.Sqrt(0.2); math.Abs(got-want) > 1e-12 {
		t.Fatalf("crowded 200M intensity=%g want=%g", got, want)
	}
}

func TestMagnitudeScaleIsAbsoluteContinuousAndOnlyCapsAtOneBillion(t *testing.T) {
	scale := newMagnitudeScale(nil, nil)
	cases := []struct {
		value int64
		want  float64
	}{
		{0, 0},
		{10_000_000, 0.1},
		{50_000_000, math.Sqrt(0.05)},
		{100_000_000, math.Sqrt(0.1)},
		{200_000_000, math.Sqrt(0.2)},
		{400_000_000, math.Sqrt(0.4)},
		{600_000_000, math.Sqrt(0.6)},
		{800_000_000, math.Sqrt(0.8)},
		{863_000_000, math.Sqrt(0.863)},
		{1_000_000_000, 1},
		{10_000_000_000, 1},
	}
	previous := -1.0
	for _, tc := range cases {
		got := scale.intensity(tc.value)
		if math.Abs(got-tc.want) > 1e-12 {
			t.Fatalf("value=%d intensity=%g want=%g", tc.value, got, tc.want)
		}
		if got < previous {
			t.Fatalf("intensity is not monotonic at value=%d: %g < %g", tc.value, got, previous)
		}
		previous = got
	}
	if got := scale.intensity(863_000_000) - scale.intensity(765_000_000); got < 0.05 {
		t.Fatalf("765M and 863M are not materially separated: delta=%g", got)
	}
}

func TestPreviewIsTransparentDenseAndUncropped(t *testing.T) {
	snap := previewFixture(t)
	var out bytes.Buffer
	if err := RenderPreview(&out, snap, ThemeLightGreen); err != nil {
		t.Fatal(err)
	}
	svg := out.String()
	if !strings.Contains(svg, `viewBox="0 0 800 194"`) {
		t.Fatal("preview must use the compact 800x194 viewBox")
	}
	if strings.Contains(svg, `<rect x="0.5" y="0.5"`) || strings.Contains(svg, `#F7F6F3`) {
		t.Fatal("light preview must not draw the old beige outer card")
	}
	if strings.Contains(svg, "53 weeks") {
		t.Fatal("month ticks make the 53 weeks label redundant")
	}
	if got := strings.Count(svg, `class="day"`); got != WallDays {
		t.Fatalf("wall cells=%d want=%d", got, WallDays)
	}
	if !strings.Contains(svg, `class="day" x="28.0" y="88.0" width="13.0" height="13.0"`) {
		t.Fatal("first calendar cell is not at the intended wall origin")
	}
	if !strings.Contains(svg, `class="day" x="782.0"`) {
		t.Fatal("last calendar column is missing or cropped")
	}
	if !strings.Contains(svg, `y="175.0" width="13.0" height="13.0"`) {
		t.Fatal("last calendar row is missing or cropped")
	}
	assertRectsInsideViewBox(t, svg, 800, 194)
	for _, label := range []string{">Mon</text>", ">Wed</text>", ">Fri</text>"} {
		if !strings.Contains(svg, label) {
			t.Fatalf("weekday label %s missing", label)
		}
	}
}

func TestPreviewMonthTicksDoNotOverlap(t *testing.T) {
	snap := previewFixture(t)
	ticks := monthTicks(snap.Activity.Dates)
	if len(ticks) < 10 {
		t.Fatalf("month tick count=%d want at least 10", len(ticks))
	}
	seenSep := false
	for i, tick := range ticks {
		if tick.label == "Sep" {
			seenSep = true
		}
		if tick.x < wallX || tick.x+tick.width > previewW {
			t.Fatalf("month %s is outside preview: x=%g width=%g", tick.label, tick.x, tick.width)
		}
		if i > 0 {
			prev := ticks[i-1]
			if tick.x < prev.x+prev.width+monthTickGap {
				t.Fatalf("month labels overlap: %s ends at %g, %s starts at %g", prev.label, prev.x+prev.width, tick.label, tick.x)
			}
		}
	}
	if !seenSep {
		t.Fatal("expected visible Sep month tick")
	}
}

func TestPreviewWeekdayTicksFitTheExistingGutter(t *testing.T) {
	for _, tick := range weekdayTicks() {
		if tick.x-tick.width < 0 || tick.x >= wallX {
			t.Fatalf("weekday %s does not fit the left gutter: x=%g width=%g", tick.label, tick.x, tick.width)
		}
		if tick.y < wallY || tick.y > wallY+7*wallCell+6*wallGap {
			t.Fatalf("weekday %s is outside the wall height: y=%g", tick.label, tick.y)
		}
	}
}

func TestPreviewContinuousColorsIgnoreCompatibilityLevels(t *testing.T) {
	snap := previewFixture(t)
	ser := allTokenSeries(&snap)
	for i := range ser.States {
		ser.States[i] = CellEmpty
		ser.Values[i] = 0
		ser.Levels[i] = 0
	}
	ser.States[0], ser.Values[0], ser.Levels[0] = CellActive, 200_000_000, 5
	ser.States[1], ser.Values[1], ser.Levels[1] = CellActive, 800_000_000, 5
	for i := 2; i < 102; i++ {
		ser.States[i] = CellActive
		ser.Values[i] = int64(i-1) * 10_000_000
		ser.Levels[i] = 5
	}
	ser.States[102], ser.Values[102], ser.Levels[102] = CellActive, 10_000_000_000, 5

	scale := newMagnitudeScale(ser.Values, ser.States)
	color200 := heatColor(ThemeLightGreen.Heat, scale.intensity(200_000_000))
	color800 := heatColor(ThemeLightGreen.Heat, scale.intensity(800_000_000))
	if color200 == color800 {
		t.Fatalf("equal legacy levels collapsed distinct raw values to %s", color200)
	}
	if relativeLuminance(color800) >= relativeLuminance(color200) {
		t.Fatalf("larger value must render stronger/darker: 200M=%s 800M=%s", color200, color800)
	}
}

func TestRichHeatRampsAreGamutSafeAndBecomeColorfulEarly(t *testing.T) {
	for _, theme := range []Theme{ThemeLightGreen, ThemeLightCobalt, ThemeLightIndigo, ThemeLightMagenta} {
		ramp := theme.Heat
		stops := []OKLCH{ramp.Low, ramp.Mid, ramp.High, ramp.Max}
		for i, color := range stops {
			if !inSRGBGamut(color) {
				t.Fatalf("%s stop %d is outside sRGB gamut: %+v", theme.Name, i, color)
			}
			if i > 0 && color.C < stops[i-1].C {
				t.Fatalf("%s chroma weakens at stop %d: %g < %g", theme.Name, i, color.C, stops[i-1].C)
			}
		}
		at10M := ramp.colorAt(0.1)
		at200M := ramp.colorAt(math.Sqrt(0.2))
		at400M := ramp.colorAt(math.Sqrt(0.4))
		at800M := ramp.colorAt(math.Sqrt(0.8))
		if at10M.C <= ramp.Low.C || at200M.C < ramp.Low.C+0.04 || at400M.C <= at200M.C || at800M.C <= at400M.C {
			t.Fatalf("%s does not gain chroma early and continuously: 10M=%g 200M=%g 400M=%g 800M=%g", theme.Name, at10M.C, at200M.C, at400M.C, at800M.C)
		}
		naiveAtMid := ramp.Low.C + 0.25*(ramp.Max.C-ramp.Low.C)
		if ramp.colorAt(heatMidPosition).C-naiveAtMid < 0.02 {
			t.Fatalf("%s mid stop is effectively a linear endpoint interpolation", theme.Name)
		}
	}
}

func TestNewsprintPreviewUsesTheSameAbsoluteInkScale(t *testing.T) {
	snap := previewFixture(t)
	ser := allTokenSeries(&snap)
	for i := range ser.States {
		ser.States[i], ser.Values[i] = CellEmpty, 0
	}
	ser.States[0], ser.Values[0] = CellActive, 200_000_000
	ser.States[1], ser.Values[1] = CellActive, 800_000_000

	var out bytes.Buffer
	if err := RenderPreview(&out, snap, ThemeLightNewsprint); err != nil {
		t.Fatal(err)
	}
	svg := out.String()
	if !strings.Contains(svg, `id="newsprint-ink-`) {
		t.Fatal("newsprint preview must include its subtle ink texture definition")
	}
	if got, want := strings.Count(svg, `fill="url(#newsprint-ink-`), 2; got != want {
		t.Fatalf("newsprint active cells using ink texture=%d want=%d", got, want)
	}
	if ThemeLightNewsprint.Heat.Low.C != 0 || ThemeLightNewsprint.Heat.Max.C != 0 {
		t.Fatal("newsprint preview must remain monochrome")
	}
	if ThemeLightNewsprint.Heat.colorAt(math.Sqrt(.8)).L >= ThemeLightNewsprint.Heat.colorAt(math.Sqrt(.2)).L {
		t.Fatal("800M newsprint ink must be darker than 200M ink")
	}
	if strings.Contains(svg, `fill="#fff" fill-opacity=".14"`) {
		t.Fatal("newsprint ink must not use the old halftone dot")
	}
}

func TestNewsprintDarkPreviewLightensWithActivity(t *testing.T) {
	low := ThemeDarkNewsprint.Heat.colorAt(math.Sqrt(0.2)).L
	high := ThemeDarkNewsprint.Heat.colorAt(math.Sqrt(0.8)).L
	if high <= low {
		t.Fatalf("dark newsprint must lighten toward larger days: low=%g high=%g", low, high)
	}
	for _, theme := range []Theme{ThemeDarkCobalt, ThemeDarkMagenta, ThemeDarkNewsprint} {
		for _, intensity := range []float64{0, heatMidPosition, heatHighPosition, 1} {
			if !inSRGBGamut(gamutMapToSRGB(theme.Heat.colorAt(intensity))) {
				t.Fatalf("%s intensity %g leaves sRGB", theme.Name, intensity)
			}
		}
	}
	light, dark := ThemesFor(PaletteCobalt)
	if light.Name == dark.Name || dark.Text == light.Text {
		t.Fatal("cobalt dark preview must be its own theme")
	}
}

func TestRenderRampStripShowsEmptyAndFourActiveStops(t *testing.T) {
	var out bytes.Buffer
	if err := RenderRampStrip(&out, ThemeLightCobalt); err != nil {
		t.Fatal(err)
	}
	svg := out.String()
	if strings.Count(svg, `class="ramp-step"`) != 5 {
		t.Fatal("ramp strip must include empty, low, medium, high, and max")
	}
	if !strings.Contains(svg, ThemeLightCobalt.Empty) {
		t.Fatal("ramp strip must retain the neutral empty cell")
	}
	for _, intensity := range []float64{0, heatMidPosition, heatHighPosition, 1} {
		if !strings.Contains(svg, heatColor(ThemeLightCobalt.Heat, intensity)) {
			t.Fatalf("ramp strip missing rendered active color at intensity=%g", intensity)
		}
	}
}

func TestPreviewPalettesShareNeutralsAndUseOneHueFamily(t *testing.T) {
	base := ThemeLightGreen
	for _, candidate := range []Theme{ThemeLightCobalt, ThemeLightIndigo, ThemeLightMagenta} {
		if candidate.Text != base.Text || candidate.Secondary != base.Secondary || candidate.Empty != base.Empty || candidate.Future != base.Future || candidate.Unknown != base.Unknown || candidate.Hairline != base.Hairline {
			t.Fatalf("candidate %s changes non-heat neutrals", candidate.Name)
		}
		for _, color := range []OKLCH{candidate.Heat.Low, candidate.Heat.Mid, candidate.Heat.High, candidate.Heat.Max} {
			if color.H != candidate.Heat.Low.H {
				t.Fatalf("candidate %s uses more than one hue", candidate.Name)
			}
		}
	}
}

func TestFromSnapshotParity(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{ev("claude", "anthropic", now.Add(-time.Hour), 100, 20, 5)}
	snap, err := Build(Input{Events: events, Now: now, Loc: loc, IncludeCost: true})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Periods.All.Totals.Total.Value == nil {
		t.Fatal("total")
	}
	if *snap.Periods.All.Totals.Total.Value != 125 {
		t.Fatalf("total=%d", *snap.Periods.All.Totals.Total.Value)
	}
}

func previewFixture(t *testing.T) Snapshot {
	t.Helper()
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{
		Events: []event.UsageEvent{ev("claude", "anthropic", now.Add(-time.Hour), 1000, 0, 10)},
		Now:    now, Loc: loc, Version: "v0.7.2-dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func allTokenSeries(snap *Snapshot) *Series {
	for i := range snap.Activity.Series {
		ser := &snap.Activity.Series[i]
		if ser.Dimension == "all" && (ser.Metric == MetricTokens || ser.Metric == "") {
			return ser
		}
	}
	return nil
}

func assertRectsInsideViewBox(t *testing.T, svg string, width, height float64) {
	t.Helper()
	re := regexp.MustCompile(`<rect[^>]*x="([0-9.]+)"[^>]*y="([0-9.]+)"[^>]*width="([0-9.]+)"[^>]*height="([0-9.]+)"`)
	for _, match := range re.FindAllStringSubmatch(svg, -1) {
		x, _ := strconv.ParseFloat(match[1], 64)
		y, _ := strconv.ParseFloat(match[2], 64)
		w, _ := strconv.ParseFloat(match[3], 64)
		h, _ := strconv.ParseFloat(match[4], 64)
		if x < 0 || y < 0 || x+w > width || y+h > height {
			t.Fatalf("cropped rect: x=%g y=%g width=%g height=%g", x, y, w, h)
		}
	}
}

func relativeLuminance(hex string) float64 {
	if len(hex) != 7 || hex[0] != '#' {
		return math.NaN()
	}
	component := func(part string) float64 {
		n, _ := strconv.ParseUint(part, 16, 8)
		v := float64(n) / 255
		if v <= 0.04045 {
			return v / 12.92
		}
		return math.Pow((v+0.055)/1.055, 2.4)
	}
	return 0.2126*component(hex[1:3]) + 0.7152*component(hex[3:5]) + 0.0722*component(hex[5:7])
}
