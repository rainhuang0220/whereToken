package profilewebembed

import (
	"bytes"
	"image"
	"image/png"
	"strings"
	"testing"
)

func TestLivePageHasNoDashboardAPIs(t *testing.T) {
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(js)
	for _, bad := range []string{"/api/summary", "/api/v1/", "wheretoken login", "wheretoken sync", "community"} {
		if strings.Contains(s, bad) {
			t.Fatalf("forbidden %q", bad)
		}
	}
	if !strings.Contains(s, "./profile.json") {
		t.Fatal("must fetch ./profile.json")
	}
}

func TestNewsprintMaterialAssetsAreEmbedded(t *testing.T) {
	surface, err := Read("assets/newsprint-surface.png")
	if err != nil {
		t.Fatal(err)
	}
	if len(surface) > 700*1024 {
		t.Fatalf("newsprint surface is %d bytes; want no more than 700 KiB", len(surface))
	}
	decoded, err := png.Decode(bytes.NewReader(surface))
	if err != nil {
		t.Fatalf("decode newsprint surface: %v", err)
	}
	if got := decoded.Bounds().Dx(); got != 1920 {
		t.Fatalf("surface width=%d want 1920", got)
	}
	if got := decoded.Bounds().Dy(); got != 2400 {
		t.Fatalf("surface height=%d want 2400", got)
	}
	if _, ok := decoded.(*image.Gray); !ok {
		t.Fatalf("surface must be 8-bit grayscale, got %T", decoded)
	}
	seen := [256]bool{}
	levels := 0
	gray := decoded.(*image.Gray)
	for _, v := range gray.Pix {
		if !seen[v] {
			seen[v] = true
			levels++
		}
	}
	if levels < 32 {
		t.Fatalf("surface only uses %d gray levels; 4-level quantization still present?", levels)
	}

	for _, name := range []string{"assets/newsprint-fiber.svg", "assets/newsprint-fiber-b.svg"} {
		fiber, err := Read(name)
		if err != nil {
			t.Fatal(err)
		}
		if len(fiber) > 20*1024 {
			t.Fatalf("%s is %d bytes; want no more than 20 KiB", name, len(fiber))
		}
		s := string(fiber)
		for _, bad := range []string{`href="http://`, `href="https://`, `href='http://`, `href='https://`, "WebGL", "three.js"} {
			if strings.Contains(s, bad) {
				t.Fatalf("%s must not load external or runtime shader asset %q", name, bad)
			}
		}
	}
}

func TestNewsprintDoesNotRewriteOtherPalettesOrIntensity(t *testing.T) {
	css, err := Read("assets/profile.css")
	if err != nil {
		t.Fatal(err)
	}
	s := string(css)
	for _, want := range []string{
		`--data-accent: #ffd700;`,
		`--theme-label: #0969da;`,
		`--data-accent: #c2185b;`,
		`--theme-label: #c2185b;`,
		`background-size: 131px 127px, 173px 149px, 1920px 2400px;`,
		`./newsprint-fiber-b.svg`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("stylesheet missing %q", want)
		}
	}
	for _, bad := range []string{"background-size: 100% 100%", "newsprint-folds.svg", "--paper-crease", "WebGL", "three.js"} {
		if strings.Contains(s, bad) {
			t.Fatalf("stylesheet contains forbidden %q", bad)
		}
	}

	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	script := string(js)
	if !strings.Contains(script, "const ABSOLUTE_TOKEN_CAP = 1_000_000_000") {
		t.Fatal("absolute token cap changed")
	}
	if !strings.Contains(script, "return Math.sqrt(Math.min(Math.max(Number(value) || 0, 0) / ABSOLUTE_TOKEN_CAP, 1));") {
		t.Fatal("absolute token intensity formula changed")
	}
	for _, bad := range []string{"WebGL", "webgl", "three.js", "THREE.", "fragmentShader"} {
		if strings.Contains(script, bad) {
			t.Fatalf("profile script contains runtime shader dependency %q", bad)
		}
	}
}
