package profilewebembed

import (
	"bytes"
	"image"
	"image/jpeg"
	"strings"
	"testing"
)

func TestLivePageHasNoDashboardAPIs(t *testing.T) {
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(js)
	for _, bad := range []string{"/api/summary", "/api/v1/sync", "/api/v1/dashboard", "/api/v1/account", "/api/v1/devices", "wheretoken login", "wheretoken sync", "community"} {
		if strings.Contains(s, bad) {
			t.Fatalf("forbidden %q", bad)
		}
	}
	if !strings.Contains(s, "/api/v1/public-profile/") {
		t.Fatal("live page does not request the hosted public profile")
	}
	if !strings.Contains(s, "./profile.json") {
		t.Fatal("must fetch ./profile.json")
	}
}

func TestNewsprintMaterialAssetsAreEmbedded(t *testing.T) {
	surface, err := Read("assets/newsprint-surface.jpg")
	if err != nil {
		t.Fatal(err)
	}
	if len(surface) > 250*1024 {
		t.Fatalf("newsprint surface is %d bytes; want no more than 250 KiB", len(surface))
	}
	decoded, err := jpeg.Decode(bytes.NewReader(surface))
	if err != nil {
		t.Fatalf("decode newsprint surface: %v", err)
	}
	if got := decoded.Bounds().Dx(); got != 1280 {
		t.Fatalf("surface width=%d want 1280", got)
	}
	if got := decoded.Bounds().Dy(); got != 798 {
		t.Fatalf("surface height=%d want 798", got)
	}
	if _, ok := decoded.(*image.YCbCr); !ok {
		t.Fatalf("surface must be a JPEG, got %T", decoded)
	}

	for _, stale := range []string{"assets/newsprint-fiber.svg", "assets/newsprint-fiber-b.svg", "assets/newsprint-surface.png", "assets/newsprint-folds.svg"} {
		if _, err := Read(stale); err == nil {
			t.Fatalf("stale newsprint asset still embedded: %s", stale)
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
		`background-size: 560px auto;`,
		`./newsprint-surface.jpg`,
		`--page: #f7f6f1;`,
	} {
		if !strings.Contains(s, want) {
			t.Fatalf("stylesheet missing %q", want)
		}
	}
	for _, bad := range []string{"background-size: 100% 100%", "newsprint-folds.svg", "newsprint-fiber.svg", "--paper-crease", "WebGL", "three.js", "feTurbulence"} {
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
