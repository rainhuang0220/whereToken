package publicprofile

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math"
	"regexp"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/profilewebembed"
)

type wallStop struct {
	At      float64
	L, C, H float64
}

func (s *wallStop) UnmarshalJSON(raw []byte) error {
	var pair []json.RawMessage
	if err := json.Unmarshal(raw, &pair); err != nil {
		return err
	}
	if len(pair) != 2 {
		return fmt.Errorf("palette stop wants [position, color]")
	}
	if err := json.Unmarshal(pair[0], &s.At); err != nil {
		return err
	}
	var color [3]float64
	if err := json.Unmarshal(pair[1], &color); err != nil {
		return err
	}
	s.L, s.C, s.H = color[0], color[1], color[2]
	return nil
}

type wallRamp struct {
	Empty   string     `json:"empty"`
	Future  string     `json:"future"`
	Unknown string     `json:"unknown"`
	Stops   []wallStop `json:"stops"`
}

type wallPaletteFile struct {
	Palettes map[string]struct {
		Label   string   `json:"label"`
		Texture bool     `json:"texture"`
		Light   wallRamp `json:"light"`
		Dark    wallRamp `json:"dark"`
	} `json:"palettes"`
}

func loadWallPaletteTokens(t *testing.T) wallPaletteFile {
	t.Helper()
	js, err := profilewebembed.Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	const start = "const WALL_PALETTE_TOKENS = JSON.parse(`"
	const end = "`);"
	script := string(js)
	i := strings.Index(script, start)
	if i < 0 {
		t.Fatal("profile.js is missing WALL_PALETTE_TOKENS")
	}
	rest := script[i+len(start):]
	j := strings.Index(rest, end)
	if j < 0 {
		t.Fatal("profile.js palette token block is not closed")
	}
	var file wallPaletteFile
	if err := json.Unmarshal([]byte(rest[:j]), &file); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(script, "return Math.sqrt(Math.min(Math.max(Number(value) || 0, 0) / ABSOLUTE_TOKEN_CAP, 1));") {
		t.Fatal("web token intensity is not the absolute square-root scale")
	}
	return file
}

func assertRamp(t *testing.T, name string, got wallRamp, th Theme) {
	t.Helper()
	if !strings.EqualFold(got.Empty, th.Empty) || !strings.EqualFold(got.Future, th.Future) || !strings.EqualFold(got.Unknown, th.Unknown) {
		t.Fatalf("%s neutrals web empty=%s future=%s unknown=%s svg empty=%s future=%s unknown=%s", name, got.Empty, got.Future, got.Unknown, th.Empty, th.Future, th.Unknown)
	}
	wantStops := []OKLCH{th.Heat.Low, th.Heat.Mid, th.Heat.High, th.Heat.Max}
	wantAt := []float64{0, heatMidPosition, heatHighPosition, 1}
	if len(got.Stops) != len(wantStops) {
		t.Fatalf("%s level count=%d want %d", name, len(got.Stops), len(wantStops))
	}
	for i, stop := range got.Stops {
		if math.Abs(stop.At-wantAt[i]) > 1e-9 || math.Abs(stop.L-wantStops[i].L) > 1e-9 || math.Abs(stop.C-wantStops[i].C) > 1e-9 || math.Abs(stop.H-wantStops[i].H) > 1e-9 {
			t.Fatalf("%s stop %d web at=%g %+v svg at=%g %+v", name, i, stop.At, OKLCH{L: stop.L, C: stop.C, H: stop.H}, wantAt[i], wantStops[i])
		}
	}
}

func TestWebPaletteTokensMatchPreviewThemes(t *testing.T) {
	file := loadWallPaletteTokens(t)
	if len(file.Palettes) != 3 {
		t.Fatalf("palette count=%d", len(file.Palettes))
	}
	for _, id := range []string{PaletteCobalt, PaletteMagenta, PaletteNewsprint} {
		if _, ok := file.Palettes[id]; !ok {
			t.Fatalf("missing palette %s", id)
		}
	}
	cobalt := file.Palettes[PaletteCobalt]
	magenta := file.Palettes[PaletteMagenta]
	news := file.Palettes[PaletteNewsprint]
	if cobalt.Texture || magenta.Texture || !news.Texture {
		t.Fatal("newsprint is the only textured palette")
	}
	lightCobalt, darkCobalt := ThemesFor(PaletteCobalt)
	lightMagenta, darkMagenta := ThemesFor(PaletteMagenta)
	assertRamp(t, "cobalt light", cobalt.Light, lightCobalt)
	assertRamp(t, "cobalt dark", cobalt.Dark, darkCobalt)
	assertRamp(t, "magenta light", magenta.Light, lightMagenta)
	assertRamp(t, "magenta dark", magenta.Dark, darkMagenta)

	// Newsprint ink stays on the light sheet. The dark SVG is a separate
	// GitHub chrome adaptation and must not become the page ramp.
	assertRamp(t, "newsprint light ink", wallRamp{
		Empty: ThemeLightNewsprint.Empty, Future: ThemeLightNewsprint.Future, Unknown: ThemeLightNewsprint.Unknown,
		Stops: news.Light.Stops,
	}, ThemeLightNewsprint)
	if len(news.Dark.Stops) != len(news.Light.Stops) {
		t.Fatal("newsprint dark sheet changed the level count")
	}
	for i, stop := range news.Dark.Stops {
		light := news.Light.Stops[i]
		if stop != light {
			t.Fatalf("newsprint dark stop %d inverted the paper ramp: %+v light %+v", i, stop, light)
		}
	}
	if ThemeDarkNewsprint.Heat.Low.L != 0.40 || ThemeDarkNewsprint.Heat.Max.L != 0.88 || ThemeDarkNewsprint.Empty != "#24211E" {
		t.Fatal("dark newsprint SVG theme changed")
	}
	css, err := profilewebembed.Read("assets/profile.css")
	if err != nil {
		t.Fatal(err)
	}
	sheet := string(css)
	if !strings.Contains(sheet, "--empty: #e8e8e6;") || !strings.Contains(sheet, "--empty: #e6e2d8;") {
		t.Fatal("newsprint paper empty colors changed")
	}
}

func TestSVGCellsMatchWebTokenColors(t *testing.T) {
	snap := previewFixture(t)
	ser := allTokenSeries(&snap)
	for i := range ser.States {
		ser.States[i] = CellEmpty
		ser.Values[i] = 0
		ser.Levels[i] = 5
	}
	ser.States[0] = CellFuture
	ser.States[1] = CellUnknown
	values := []int64{1_000_000, 10_000_000, 50_000_000, 100_000_000, 200_000_000, 400_000_000, 800_000_000, 1_000_000_000}
	for i, value := range values {
		ser.States[2+i] = CellActive
		ser.Values[2+i] = value
		ser.Levels[2+i] = 5
	}
	re := regexp.MustCompile(`<rect class="day"[^>]*fill="([^"]+)"`)
	scale := newMagnitudeScale(ser.Values, ser.States)
	for _, palette := range []string{PaletteCobalt, PaletteMagenta} {
		light, dark := ThemesFor(palette)
		for _, th := range []Theme{light, dark} {
			var buf bytes.Buffer
			if err := RenderPreview(&buf, snap, th); err != nil {
				t.Fatal(err)
			}
			fills := re.FindAllStringSubmatch(buf.String(), -1)
			if len(fills) != len(ser.States) {
				t.Fatalf("%s %s cells=%d", palette, th.Name, len(fills))
			}
			for i, match := range fills {
				want := th.Empty
				switch ser.States[i] {
				case CellFuture:
					want = th.Future
				case CellUnknown:
					want = th.Unknown
				case CellActive:
					if ser.Values[i] > 0 {
						want = heatColor(th.Heat, scale.intensity(ser.Values[i]))
					}
				}
				if !strings.EqualFold(match[1], want) {
					t.Fatalf("%s %s day %d level %d value %d fill %s want %s", palette, th.Name, i, ser.Levels[i], ser.Values[i], match[1], want)
				}
			}
		}
	}
}
