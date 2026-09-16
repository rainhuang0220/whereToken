// Command gennewsprint bakes the deterministic Newsprint material used by the
// public profile. It generates a periodic height field, derives normals, and
// lights the surface with one matte upper-left diffuse light.
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
	productionSize = 1536
	materialSeed   = uint64(0x7768657265546f6b) // "whereTok"
)

type mode struct {
	kx, ky float64
	phase  float64
	weight float64
}

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
	flag.Parse()

	assetDir := filepath.Join(*root, "internal", "profilewebembed", "static", "assets")
	surface, err := bakeSurface(productionSize, materialSeed)
	if err != nil {
		fail(err)
	}
	outputs := map[string][]byte{
		filepath.Join(assetDir, "newsprint-surface.png"):                        surface,
		filepath.Join(assetDir, "newsprint-fiber.svg"):                          fiberSVG(),
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

func bakeSurface(size int, seed uint64) ([]byte, error) {
	heights := heightField(size, seed)
	diffuse := make([]float64, size*size)
	lightX, lightY, lightZ := normalize3(-0.45, -0.55, 0.70)

	// Normalize the field's slopes before lighting so authored amplitudes stay
	// stable if the mode mix changes. All tone still comes from local orientation.
	var slopeEnergy float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := heights[y*size+(x+1)%size] - heights[y*size+(x-1+size)%size]
			dy := heights[((y+1)%size)*size+x] - heights[((y-1+size)%size)*size+x]
			slopeEnergy += dx*dx + dy*dy
		}
	}
	slopeRMS := math.Sqrt(slopeEnergy / float64(size*size))
	slopeScale := 0.075 / math.Max(slopeRMS, 1e-9)

	var mean float64
	for y := 0; y < size; y++ {
		for x := 0; x < size; x++ {
			dx := heights[y*size+(x+1)%size] - heights[y*size+(x-1+size)%size]
			dy := heights[((y+1)%size)*size+x] - heights[((y-1+size)%size)*size+x]
			nx, ny, nz := normalize3(-slopeScale*dx, -slopeScale*dy, 1)
			d := math.Max(0, nx*lightX+ny*lightY+nz*lightZ)
			diffuse[y*size+x] = d
			mean += d
		}
	}
	mean /= float64(len(diffuse))

	var variance float64
	for _, d := range diffuse {
		variance += (d - mean) * (d - mean)
	}
	stddev := math.Sqrt(variance / float64(len(diffuse)))

	img := image.NewGray(image.Rect(0, 0, size, size))
	for i, d := range diffuse {
		// A neutral soft-light map: most pixels land within 108..148 and rare
		// combined extrema are clamped to 104..152. Four-level quantization keeps
		// the static PNG compact without turning the surface into visible bands.
		v := 128 + 10.5*(d-mean)/math.Max(stddev, 1e-9)
		v = math.Round(v/4) * 4
		v = math.Max(104, math.Min(152, v))
		img.Pix[i] = uint8(v)
	}

	var out bytes.Buffer
	encoder := png.Encoder{CompressionLevel: png.BestCompression}
	if err := encoder.Encode(&out, img); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func heightField(size int, seed uint64) []float64 {
	rng := splitMix64{state: seed}
	broad := makeModes(&rng, 5, 2.2, 6.0, 0.20)
	cockle := makeModes(&rng, 27, 8.5, 46.0, 1.00)
	field := make([]float64, size*size)
	tau := 2 * math.Pi
	for y := 0; y < size; y++ {
		v := float64(y) / float64(size)
		for x := 0; x < size; x++ {
			u := float64(x) / float64(size)
			// Periodic, low-amplitude domain warp prevents parallel Fourier bands
			// without introducing a non-periodic seam or a line-shaped feature.
			warpX := 0.010*math.Sin(tau*(2*u+3*v)+0.71) + 0.006*math.Sin(tau*(-3*u+2*v)+2.19)
			warpY := 0.009*math.Sin(tau*(-2*u+3*v)+1.37) + 0.006*math.Sin(tau*(4*u+v)+4.11)
			wu, wv := u+warpX, v+warpY
			value := sumModes(broad, wu, wv, tau) + sumModes(cockle, wu, wv, tau)
			field[y*size+x] = value
		}
	}
	return field
}

func makeModes(rng *splitMix64, count int, minRadius, maxRadius, bandWeight float64) []mode {
	modes := make([]mode, 0, count)
	for len(modes) < count {
		radius := minRadius + (maxRadius-minRadius)*rng.unit()
		angle := 2 * math.Pi * rng.unit()
		kx := math.Round(radius * math.Cos(angle))
		ky := math.Round(radius * math.Sin(angle))
		if kx == 0 && ky == 0 {
			continue
		}
		actualRadius := math.Hypot(kx, ky)
		weight := bandWeight * (minRadius / actualRadius) * (0.72 + 0.56*rng.unit())
		modes = append(modes, mode{kx: kx, ky: ky, phase: 2 * math.Pi * rng.unit(), weight: weight})
	}
	return modes
}

func sumModes(modes []mode, u, v, tau float64) float64 {
	var value float64
	for _, m := range modes {
		value += m.weight * math.Sin(tau*(m.kx*u+m.ky*v)+m.phase)
	}
	return value
}

func normalize3(x, y, z float64) (float64, float64, float64) {
	length := math.Sqrt(x*x + y*y + z*z)
	return x / length, y / length, z / length
}

func fiberSVG() []byte {
	return []byte(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128" viewBox="0 0 128 128">
  <filter id="fiber" x="0" y="0" width="100%" height="100%" color-interpolation-filters="sRGB">
    <feTurbulence type="fractalNoise" baseFrequency="0.075 0.42" numOctaves="2" seed="7341" stitchTiles="stitch" result="height"/>
    <feGaussianBlur in="height" stdDeviation="0.18 0.48" result="soft-height"/>
    <feDiffuseLighting in="soft-height" surfaceScale="0.55" diffuseConstant="0.72" lighting-color="#808080" result="lit">
      <feDistantLight azimuth="315" elevation="58"/>
    </feDiffuseLighting>
    <feComponentTransfer in="lit">
      <feFuncR type="linear" slope="0.24" intercept="0.38"/>
      <feFuncG type="linear" slope="0.24" intercept="0.38"/>
      <feFuncB type="linear" slope="0.24" intercept="0.38"/>
      <feFuncA type="linear" slope="0.52"/>
    </feComponentTransfer>
  </filter>
  <rect width="128" height="128" fill="#808080" filter="url(#fiber)"/>
</svg>
`)
}

func swatchHTML() []byte {
	return []byte(`<!doctype html>
<html lang="en"><meta charset="utf-8"><title>Newsprint material swatch</title>
<style>
*{box-sizing:border-box}html,body{margin:0;width:1200px;height:900px;overflow:hidden}
body{padding:82px;background-color:#f2f0e9;background-image:url("../../internal/profilewebembed/static/assets/newsprint-fiber.svg"),url("../../internal/profilewebembed/static/assets/newsprint-surface.png");background-size:128px 128px,1536px 1536px;background-repeat:repeat; background-blend-mode:soft-light,soft-light;color:#171717;font:14px/1.5 Georgia,serif}
.label{margin:0 0 44px;font:600 13px/1.2 ui-monospace,monospace;letter-spacing:.12em;text-transform:uppercase}.blocks{display:flex;gap:36px;align-items:flex-end}.block{width:330px;height:210px;background-image:url("../../internal/profilewebembed/static/assets/newsprint-fiber.svg");background-size:28px 28px;background-blend-mode:soft-light}.gray{background-color:#b6b5b0}.black{background-color:#4a4946}.rules{margin-top:54px}.one{border-top:1px solid #171717}.two{margin-top:30px;border-top:2px solid #171717}.copy{max-width:530px;margin-top:46px;font-size:16px}.copy strong{font-size:22px}
</style><body><p class="label">Newsprint material · deterministic PBR-lite bake · folds 0</p><div class="blocks"><div class="block gray"></div><div class="block black"></div></div><div class="rules"><div class="one"></div><div class="two"></div></div><p class="copy"><strong>whereToken local token accounting</strong><br>Matte ink sits on a continuous, lightly cockled sheet. Type remains crisp while the substrate carries the material character.</p></body></html>
`)
}
