// Command gennewsprint bakes the deterministic Newsprint material used by the
// public profile. It generates a non-repeating heterogeneous height field,
// derives normals, and lights the surface with one matte upper-left diffuse light.
package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/png"
	"math"
	"os"
	"path/filepath"
)

const (
	// 1:1 CSS pixels. v4 stored 1280×1600 and displayed it at 2560×3200, which
	// bilinear-softened the meso shape. Unique coverage is 1920×2400 CSS px.
	productionWidth  = 1920
	productionHeight = 2400
	displayWidth     = 1920
	displayHeight    = 2400
	materialSeed     = uint64(0x7768657265546f6b) // "whereTok"
	maxSurfaceBytes  = 700 * 1024
)

type splitMix64 struct{ state uint64 }

func (r *splitMix64) next() uint64 {
	r.state += 0x9e3779b97f4a7c15
	z := r.state
	z = (z ^ (z >> 30)) * 0xbf58476d1ce4e5b9
	z = (z ^ (z >> 27)) * 0x94d049bb133111eb
	return z ^ (z >> 31)
}

func (r *splitMix64) unit() float64 {
	return float64(r.next()>>11) / (1 << 53)
}

func main() {
	root := flag.String("root", ".", "repository root")
	diag := flag.Bool("diag", false, "print 4x4 region diagnostics for a 640x800 probe and exit")
	flag.Parse()

	if *diag {
		heights := heightField(640, 800, materialSeed)
		printRegionDiagnostics(os.Stdout, 640, 800, regionStats(heights, 640, 800, 4, 4))
		payload, err := lightAndEncode(append([]float64(nil), heights...), 640, 800)
		if err != nil {
			fail(err)
		}
		fmt.Printf("probe PNG 640x800 = %d bytes; %dx%d linear estimate ~ %d bytes\n", len(payload), productionWidth, productionHeight, len(payload)*9)
		return
	}

	assetDir := filepath.Join(*root, "internal", "profilewebembed", "static", "assets")
	surface, heights, err := bakeSurface(productionWidth, productionHeight, materialSeed)
	if err != nil {
		fail(err)
	}
	if len(surface) > maxSurfaceBytes {
		fail(fmt.Errorf("surface PNG is %d bytes; budget is %d", len(surface), maxSurfaceBytes))
	}
	printRegionDiagnostics(os.Stdout, productionWidth, productionHeight, regionStats(heights, productionWidth, productionHeight, 4, 4))
	outputs := map[string][]byte{
		filepath.Join(assetDir, "newsprint-surface.png"):                        surface,
		filepath.Join(assetDir, "newsprint-fiber.svg"):                          fiberPrimary(),
		filepath.Join(assetDir, "newsprint-fiber-b.svg"):                        fiberSecondary(),
		filepath.Join(*root, "scripts", "gennewsprint", "material-swatch.html"): swatchHTML(),
	}
	for name, payload := range outputs {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(name, payload, 0o644); err != nil {
			fail(err)
		}
		digest := sha256.Sum256(payload)
		fmt.Printf("wrote %s (%d bytes, sha256:%x)\n", name, len(payload), digest)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gennewsprint:", err)
	os.Exit(1)
}

func bakeSurface(width, height int, seed uint64) ([]byte, []float64, error) {
	heights := heightField(width, height, seed)
	png, err := lightAndEncode(heights, width, height)
	if err != nil {
		return nil, nil, err
	}
	return png, heights, nil
}

func lightAndEncode(heights []float64, width, height int) ([]byte, error) {
	blurHeight(heights, width, height)
	diffuse := make([]float64, width*height)
	lightX, lightY, lightZ := normalize3(-0.45, -0.55, 0.70)

	var slopeEnergy float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx := heightAt(heights, width, height, x+1, y) - heightAt(heights, width, height, x-1, y)
			dy := heightAt(heights, width, height, x, y+1) - heightAt(heights, width, height, x, y-1)
			slopeEnergy += dx*dx + dy*dy
		}
	}
	slopeRMS := math.Sqrt(slopeEnergy / float64(width*height))
	// v3 used 0.075 after global RMS normalize. +33% keeps regional contrast
	// while making local slopes slightly more nonlinear under Lambert.
	slopeScale := 0.14 / math.Max(slopeRMS, 1e-9)

	var mean float64
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			dx := heightAt(heights, width, height, x+1, y) - heightAt(heights, width, height, x-1, y)
			dy := heightAt(heights, width, height, x, y+1) - heightAt(heights, width, height, x, y-1)
			nx, ny, nz := normalize3(-slopeScale*dx, -slopeScale*dy, 1)
			d := math.Max(0, nx*lightX+ny*lightY+nz*lightZ)
			diffuse[y*width+x] = d
			mean += d
		}
	}
	mean /= float64(len(diffuse))

	var variance float64
	for _, d := range diffuse {
		variance += (d - mean) * (d - mean)
	}
	stddev := math.Sqrt(variance / float64(len(diffuse)))

	img := image.NewGray(image.Rect(0, 0, width, height))
	for i, d := range diffuse {
		v := 128 + 21.0*(d-mean)/math.Max(stddev, 1e-9)
		v = math.Max(92, math.Min(164, v))
		img.Pix[i] = uint8(math.Round(v))
	}

	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func blurHeight(heights []float64, width, height int) {
	tmp := make([]float64, len(heights))
	// Light 3-tap lowpass. Fiber-scale sparkle stays in the SVG; meso slopes stay in the PNG.
	kernel := [3]float64{1, 2, 1}
	for y := 0; y < height; y++ {
		row := y * width
		for x := 0; x < width; x++ {
			var sum float64
			for i, w := range kernel {
				sum += w * heightAt(heights, width, height, x+i-1, y)
			}
			tmp[row+x] = sum / 4
		}
	}
	for x := 0; x < width; x++ {
		for y := 0; y < height; y++ {
			var sum float64
			for i, w := range kernel {
				sum += w * tmp[clampIndex(y+i-1, height)*width+x]
			}
			heights[y*width+x] = sum / 4
		}
	}
}

func clampIndex(v, n int) int {
	if v < 0 {
		return 0
	}
	if v >= n {
		return n - 1
	}
	return v
}

func heightAt(heights []float64, width, height, x, y int) float64 {
	if x < 0 {
		x = 0
	}
	if y < 0 {
		y = 0
	}
	if x >= width {
		x = width - 1
	}
	if y >= height {
		y = height - 1
	}
	return heights[y*width+x]
}

func normalize3(x, y, z float64) (float64, float64, float64) {
	length := math.Sqrt(x*x + y*y + z*z)
	return x / length, y / length, z / length
}
