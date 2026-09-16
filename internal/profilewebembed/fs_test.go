package profilewebembed

import (
	"bytes"
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
	if len(surface) > 250*1024 {
		t.Fatalf("newsprint surface is %d bytes; want no more than 250 KiB", len(surface))
	}
	image, err := png.Decode(bytes.NewReader(surface))
	if err != nil {
		t.Fatalf("decode newsprint surface: %v", err)
	}
	if got := image.Bounds().Dx(); got != 1536 {
		t.Fatalf("surface width=%d want 1536", got)
	}
	if got := image.Bounds().Dy(); got != 1536 {
		t.Fatalf("surface height=%d want 1536", got)
	}

	fiber, err := Read("assets/newsprint-fiber.svg")
	if err != nil {
		t.Fatal(err)
	}
	if len(fiber) > 20*1024 {
		t.Fatalf("newsprint fiber is %d bytes; want no more than 20 KiB", len(fiber))
	}
	s := string(fiber)
	for _, bad := range []string{`href="http://`, `href="https://`, `href='http://`, `href='https://`} {
		if strings.Contains(s, bad) {
			t.Fatalf("newsprint fiber must not load external asset %q", bad)
		}
	}
}
