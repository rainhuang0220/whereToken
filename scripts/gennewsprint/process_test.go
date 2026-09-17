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

func TestHeightFieldIsUnitInterval(t *testing.T) {
	t.Parallel()
	w, h := 24, 16
	disp := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			disp[y*w+x] = float64(80 + (x+y)%90)
		}
	}
	got := heightField(disp, w, h)
	var min, max, sum float64
	min = 1
	for _, v := range got {
		if v < min {
			min = v
		}
		if v > max {
			max = v
		}
		sum += v
	}
	if min < 0 || max > 1 {
		t.Fatalf("height range [%g, %g] is outside 0..1", min, max)
	}
	mean := sum / float64(len(got))
	if math.Abs(mean-0.5) > 0.08 {
		t.Fatalf("height mean=%g want near 0.5", mean)
	}
}

func TestReconstructNormalsFacesLeftOnARightRamp(t *testing.T) {
	t.Parallel()
	w, h := 32, 24
	height := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			height[y*w+x] = float64(x) / float64(w-1)
		}
	}
	nx, ny, nz := reconstructNormals(height, w, h, 0.2)
	// Ignore the periodic seam; a right-rising interior facet faces -X.
	i := 12*w + 16
	if nx[i] >= 0 {
		t.Fatalf("right-rising ramp should face left, nx=%g", nx[i])
	}
	if math.Abs(ny[i]) > 0.03 {
		t.Fatalf("horizontal ramp leaked a Y tilt: %g", ny[i])
	}
	if nz[i] <= 0.8 {
		t.Fatalf("newsprint slope is too steep, nz=%g", nz[i])
	}
}

func TestRoughnessIsHigherInValleys(t *testing.T) {
	t.Parallel()
	w, h := 20, 20
	height := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			if x > 6 && x < 14 && y > 6 && y < 14 {
				height[y*w+x] = 0.18
			} else {
				height[y*w+x] = 0.72
			}
		}
	}
	rough := roughnessField(height, w, h)
	valley := rough[10*w+10]
	ridge := rough[1*w+1]
	if valley <= ridge {
		t.Fatalf("valley roughness %g should exceed ridge %g", valley, ridge)
	}
}

func TestShadeRespondsToDeskKey(t *testing.T) {
	t.Parallel()
	w, h := 48, 32
	left := litRampMean(w, h, true)
	right := litRampMean(w, h, false)
	if left <= right {
		t.Fatalf("left-facing paper %g should be brighter under the desk key than right-facing %g", left, right)
	}
}

func TestWrapSampleIsPeriodic(t *testing.T) {
	t.Parallel()
	src := []float64{1, 2, 3, 4, 5, 6}
	if sample(src, 3, 2, -1, 0) != 3 || sample(src, 3, 2, 3, 0) != 1 {
		t.Fatal("x wrap failed")
	}
	if sample(src, 3, 2, 0, -1) != 4 || sample(src, 3, 2, 1, 2) != 2 {
		t.Fatal("y wrap failed")
	}
}

func TestNewsprintHeightPrefersFormationOverTooth(t *testing.T) {
	t.Parallel()
	w, h := 64, 48
	disp := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			tooth := float64((x + y) % 2) * 80
			roll := 48 * float64(x) / float64(w-1)
			disp[y*w+x] = 100 + tooth + roll
		}
	}
	got := heightField(disp, w, h)
	var adj float64
	var n int
	for y := 0; y < h; y++ {
		for x := 1; x < w; x++ {
			adj += math.Abs(got[y*w+x] - got[y*w+x-1])
			n++
		}
	}
	if meanAdj := adj / float64(n); meanAdj > 0.085 {
		t.Fatalf("calendered newsprint still has drawing-paper tooth: adjacent mean %g", meanAdj)
	}
	var left, right float64
	var c int
	for y := 8; y < h-8; y++ {
		left += got[y*w+6]
		right += got[y*w+w-7]
		c++
	}
	if right <= left {
		t.Fatalf("sheet formation roll was lost: left %g right %g", left/float64(c), right/float64(c))
	}
}

func TestBakeFromVendorIsDeterministicAndBudgeted(t *testing.T) {
	dir := t.TempDir()
	colorPath := filepath.Join(dir, "color.jpg")
	dispPath := filepath.Join(dir, "disp.jpg")
	writeRampJPEG(t, colorPath, 48, 32, false)
	writeRampJPEG(t, dispPath, 48, 32, true)

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

func TestSwatchUsesPrintedSheetNotProceduralLayers(t *testing.T) {
	html := string(swatchHTML())
	for _, want := range []string{"newsprint-surface.jpg", "560px auto", "#f7f6f1", "height / normal / roughness / ink"} {
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

func litRampMean(w, h int, leftFacing bool) float64 {
	height := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			u := 0.35 + 0.30*float64(x)/float64(w-1)
			if leftFacing {
				height[y*w+x] = u
			} else {
				height[y*w+x] = 1 - u
			}
		}
	}
	nx, ny, nz := reconstructNormals(height, w, h, 12)
	rough := make([]float64, w*h)
	for i := range rough {
		rough[i] = 0.74
	}
	albedo := make([]float64, w*h*3)
	for i := 0; i < w*h; i++ {
		albedo[i*3+0] = baseR
		albedo[i*3+1] = baseG
		albedo[i*3+2] = baseB
	}
	lit := shadePaper(albedo, height, nx, ny, nz, rough, w, h)
	var sum float64
	var count int
	for y := 4; y < h-4; y++ {
		for x := 4; x < w-4; x++ {
			i := (y*w + x) * 3
			sum += (lit[i] + lit[i+1] + lit[i+2]) / 3
			count++
		}
	}
	return sum / float64(count)
}

func writeRampJPEG(t *testing.T, path string, w, h int, asHeight bool) {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			level := uint8(90 + x*3)
			if !asHeight {
				level = uint8(240 + (x+y)%8)
			}
			img.SetRGBA(x, y, color.RGBA{R: level, G: level, B: level, A: 255})
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
