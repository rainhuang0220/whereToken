package main

import (
	"bytes"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"path/filepath"
	"testing"
)

func TestUnitFiberHasZeroMeanAndTargetStd(t *testing.T) {
	t.Parallel()
	src := []float64{10, 12, 8, 20, 0, 15}
	got := unitFiber(src, len(src))
	var mean, acc float64
	for _, v := range got {
		mean += v
	}
	mean /= float64(len(got))
	if math.Abs(mean) > 1e-9 {
		t.Fatalf("mean=%g want 0", mean)
	}
	for _, v := range got {
		d := v - mean
		acc += d * d
	}
	std := math.Sqrt(acc / float64(len(got)))
	if math.Abs(std-fiberUnitStd) > 1e-9 {
		t.Fatalf("std=%g want %g", std, fiberUnitStd)
	}
}

func TestHighpassRemovesConstantField(t *testing.T) {
	t.Parallel()
	w, h := 16, 12
	src := make([]float64, w*h)
	for i := range src {
		src[i] = 180
	}
	got := highpass(src, w, h, 2)
	var energy float64
	for _, v := range got {
		energy += v * v
	}
	if energy > 1e-6 {
		t.Fatalf("constant field leaked through highpass: energy=%g", energy)
	}
}

func TestBakeFromVendorIsDeterministicAndBudgeted(t *testing.T) {
	dir := t.TempDir()
	colorPath := filepath.Join(dir, "color.jpg")
	dispPath := filepath.Join(dir, "disp.jpg")
	writeSolidJPEG(t, colorPath, 48, 32, color.RGBA{R: 244, G: 243, B: 241, A: 255})
	writeSolidJPEG(t, dispPath, 48, 32, color.RGBA{R: 140, G: 140, B: 140, A: 255})

	first, size, err := bakeFromVendor(colorPath, dispPath)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := bakeFromVendor(colorPath, dispPath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatal("bake is not deterministic")
	}
	if size.X != productionWidth {
		t.Fatalf("width=%d want %d", size.X, productionWidth)
	}
	if len(first) > maxSurfaceBytes {
		t.Fatalf("surface is %d bytes; budget %d", len(first), maxSurfaceBytes)
	}
	img, err := jpeg.Decode(bytes.NewReader(first))
	if err != nil {
		t.Fatal(err)
	}
	if img.Bounds().Dx() != size.X || img.Bounds().Dy() != size.Y {
		t.Fatalf("decoded %v want %v", img.Bounds(), size)
	}
}

func TestSwatchUsesScannedSheetNotProceduralLayers(t *testing.T) {
	html := string(swatchHTML())
	for _, want := range []string{"newsprint-surface.jpg", "560px auto", "#f7f6f1"} {
		if !bytes.Contains([]byte(html), []byte(want)) {
			t.Fatalf("swatch missing %q", want)
		}
	}
	for _, bad := range []string{"newsprint-fiber.svg", "newsprint-folds.svg", "feTurbulence", "100% 100%"} {
		if bytes.Contains([]byte(html), []byte(bad)) {
			t.Fatalf("swatch contains rejected %q", bad)
		}
	}
}

func writeSolidJPEG(t *testing.T, path string, w, h int, c color.RGBA) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.SetRGBA(x, y, c)
		}
	}
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	if err := jpeg.Encode(f, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
}
