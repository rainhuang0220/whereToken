package main

import (
	"bytes"
	"crypto/sha256"
	"image/png"
	"math"
	"strings"
	"testing"
)

func TestBakeSurfaceIsDeterministicForFixedSeed(t *testing.T) {
	t.Parallel()
	first, _, err := bakeSurface(192, 192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := bakeSurface(192, 192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("same seed produced different hashes: %x != %x", sha256.Sum256(first), sha256.Sum256(second))
	}
}

func TestBakeSurfaceChangesWithSeed(t *testing.T) {
	t.Parallel()
	first, _, err := bakeSurface(192, 192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, _, err := bakeSurface(192, 192, materialSeed+1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("different seeds produced identical surface assets")
	}
}

func TestBakeSurfaceUsesFullEightBitGray(t *testing.T) {
	t.Parallel()
	payload, _, err := bakeSurface(192, 240, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	img, err := png.Decode(bytes.NewReader(payload))
	if err != nil {
		t.Fatal(err)
	}
	b := img.Bounds()
	seen := [256]bool{}
	n := 0
	step4 := 0
	pixels := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, _, _, _ := img.At(x, y).RGBA()
			v := uint8(r >> 8)
			pixels++
			if !seen[v] {
				seen[v] = true
				n++
			}
			if v%4 == 0 {
				step4++
			}
		}
	}
	if n < 32 {
		t.Fatalf("surface only uses %d gray levels; 4-level quantization still present?", n)
	}
	if float64(step4)/float64(pixels) > 0.92 {
		t.Fatalf("too many 4-aligned gray values (%d/%d); quantization still present", step4, pixels)
	}
}

func TestHeightFieldIsRegionallyHeterogeneous(t *testing.T) {
	t.Parallel()
	const w, h = 512, 640
	heights := heightField(w, h, materialSeed)
	stats := regionStats(heights, w, h, 4, 4)
	if len(stats) != 16 {
		t.Fatalf("got %d regions, want 16", len(stats))
	}
	minH, maxH := math.MaxFloat64, 0.0
	minS, maxS := math.MaxFloat64, 0.0
	for _, s := range stats {
		if s.heightRMS < minH {
			minH = s.heightRMS
		}
		if s.heightRMS > maxH {
			maxH = s.heightRMS
		}
		if s.slopeRMS < minS {
			minS = s.slopeRMS
		}
		if s.slopeRMS > maxS {
			maxS = s.slopeRMS
		}
		if math.IsNaN(s.heightRMS) || math.IsNaN(s.slopeRMS) {
			t.Fatalf("non-finite region stat %+v", s)
		}
	}
	hRatio := maxH / math.Max(minH, 1e-9)
	sRatio := maxS / math.Max(minS, 1e-9)
	if hRatio < 1.35 {
		t.Fatalf("height RMS ratio %.3f is too stationary (min %.4f max %.4f)", hRatio, minH, maxH)
	}
	if sRatio < 1.30 {
		t.Fatalf("slope RMS ratio %.3f is too stationary (min %.4f max %.4f)", sRatio, minS, maxS)
	}
	if hRatio > 12 || sRatio > 12 {
		t.Fatalf("regional contrast is unbounded: hRatio=%.3f sRatio=%.3f", hRatio, sRatio)
	}

	corr := cropCorrelation(heights, w, h, 16, 16, 96, 400, 480, 96)
	if corr > 0.72 {
		t.Fatalf("distant 96px crops are too similar (corr=%.3f); field still looks shifted", corr)
	}
}

func TestFiberLayersAreIncommensurateAndLit(t *testing.T) {
	primary := fiberPrimary()
	secondary := fiberSecondary()
	for _, asset := range [][]byte{primary, secondary} {
		s := string(asset)
		for _, want := range []string{"feDiffuseLighting", `stitchTiles="stitch"`, "feTurbulence"} {
			if !strings.Contains(s, want) {
				t.Fatalf("fiber asset missing %q", want)
			}
		}
		for _, bad := range []string{"href=\"http://", "href=\"https://", "three.js", "WebGL"} {
			if strings.Contains(s, bad) {
				t.Fatalf("fiber asset contains forbidden %q", bad)
			}
		}
	}
	if !bytes.Contains(primary, []byte(`width="131"`)) || !bytes.Contains(primary, []byte(`height="127"`)) {
		t.Fatal("primary fiber tile must be 131x127")
	}
	if !bytes.Contains(secondary, []byte(`width="173"`)) || !bytes.Contains(secondary, []byte(`height="149"`)) {
		t.Fatal("secondary fiber tile must be 173x149")
	}
	if bytes.Equal(primary, secondary) {
		t.Fatal("fiber layers must differ")
	}
	if bytes.Contains(primary, []byte(`baseFrequency="0.075 0.42"`)) {
		t.Fatal("primary fiber still uses the v3 repeating frequency")
	}
}

func TestSwatchDoesNotStretchMaterialToViewport(t *testing.T) {
	html := string(swatchHTML())
	if strings.Contains(html, "background-size:100%") || strings.Contains(html, "100% 100%") {
		t.Fatal("swatch must not stretch the master sheet to the viewport")
	}
	want := "2560px 3200px"
	if !strings.Contains(html, want) {
		t.Fatalf("swatch missing fixed master size %q", want)
	}
	if strings.Contains(html, "newsprint-folds.svg") {
		t.Fatal("swatch must not draw fold paths")
	}
}

func cropCorrelation(h []float64, w, _, ax, ay, size, bx, by, _ int) float64 {
	n := size * size
	var meanA, meanB float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			meanA += h[(ay+y)*w+(ax+x)]
			meanB += h[(by+y)*w+(bx+x)]
		}
	}
	meanA /= float64(n)
	meanB /= float64(n)
	var num, da, db float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			va := h[(ay+y)*w+(ax+x)] - meanA
			vb := h[(by+y)*w+(bx+x)] - meanB
			num += va * vb
			da += va * va
			db += vb * vb
		}
	}
	den := math.Sqrt(da * db)
	if den == 0 {
		return 1
	}
	return num / den
}
