package main

import (
	"bytes"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
)

// Printed-paper bake. The production JPEG is a shaded RGB sheet, not a
// grayscale overlay. Stack:
//
//   height  — photoscanned displacement, calendered toward newsprint
//   normal  — wrapped finite differences of that height field
//   roughness — valleys and steep felt are broader / more absorptive
//   lighting — one soft desk key + faint fill + micro-occlusion + sheen
//   albedo  — scanned color retinted to the CSS paper base
//
// Ink is not painted into this asset. Live CSS multiplies existing inked
// pixels over this substrate so type/rules/heat reduce paper albedo.

const (
	calenderRadius = 1.05
	feltMix        = 0.38
	broadRadius    = 34.0
	broadWeight    = 0.05
	aoRadius       = 2.4
	aoGain         = 0.085
	targetSlopeRMS = 0.58
	sheenPeak      = 0.022
	albedoGain     = 0.28
	keyWeight      = 0.30
	fillWeight     = 0.05
	ambientWeight  = 0.65
	baseR          = 247.0
	baseG          = 246.0
	baseB          = 241.0
	jpegQuality    = 83
	softenRadius   = 0.32
)

func bakeFromVendor(colorPath, dispPath string) ([]byte, image.Point, error) {
	colorImg, err := loadImage(colorPath)
	if err != nil {
		return nil, image.Point{}, err
	}
	dispImg, err := loadImage(dispPath)
	if err != nil {
		return nil, image.Point{}, err
	}
	if colorImg.Bounds() != dispImg.Bounds() {
		return nil, image.Point{}, fmt.Errorf("color %s and displacement %s bounds differ", colorImg.Bounds(), dispImg.Bounds())
	}
	b := colorImg.Bounds()
	w, h := b.Dx(), b.Dy()
	_, rch, gch, bch := rgbaPlanes(colorImg)
	disp := grayPlane(dispImg)

	height := heightField(disp, w, h)
	nx, ny, nz := reconstructNormals(height, w, h, targetSlopeRMS)
	rough := roughnessField(height, w, h)
	albedo := retintAlbedo(rch, gch, bch, w, h)
	rgb := shadePaper(albedo, height, nx, ny, nz, rough, w, h)
	rgb = gaussianRGBWrap(rgb, w, h, softenRadius)

	dw := productionWidth
	dh := int(math.Round(float64(h) * float64(dw) / float64(w)))
	rgb = resizeRGB(rgb, w, h, dw, dh)
	payload, err := encodeJPEG(rgb, dw, dh, jpegQuality)
	if err != nil {
		return nil, image.Point{}, err
	}
	return payload, image.Pt(dw, dh), nil
}

func loadImage(path string) (image.Image, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	img, _, err := image.Decode(f)
	if err != nil {
		return nil, fmt.Errorf("decode %s: %w", path, err)
	}
	return img, nil
}

func rgbaPlanes(img image.Image) (luma, r, g, b []float64) {
	bd := img.Bounds()
	n := bd.Dx() * bd.Dy()
	luma = make([]float64, n)
	r = make([]float64, n)
	g = make([]float64, n)
	b = make([]float64, n)
	i := 0
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			rr, gg, bb, _ := img.At(x, y).RGBA()
			rf := float64(rr >> 8)
			gf := float64(gg >> 8)
			bf := float64(bb >> 8)
			r[i], g[i], b[i] = rf, gf, bf
			luma[i] = (rf + gf + bf) / 3
			i++
		}
	}
	return luma, r, g, b
}

func grayPlane(img image.Image) []float64 {
	bd := img.Bounds()
	out := make([]float64, bd.Dx()*bd.Dy())
	i := 0
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			rr, _, _, _ := img.At(x, y).RGBA()
			out[i] = float64(rr >> 8)
			i++
		}
	}
	return out
}

func heightField(disp []float64, w, h int) []float64 {
	raw := make([]float64, len(disp))
	for i, v := range disp {
		raw[i] = v / 255
	}
	calendered := gaussianWrap(raw, w, h, calenderRadius)
	felt := make([]float64, len(raw))
	for i := range raw {
		felt[i] = calendered[i]*(1-feltMix) + raw[i]*feltMix
	}
	broad := gaussianWrap(raw, w, h, broadRadius)
	broadMean := meanOf(broad)
	out := make([]float64, len(raw))
	for i := range raw {
		out[i] = felt[i] + (broad[i]-broadMean)*broadWeight
	}
	return unit01(out)
}

func unit01(src []float64) []float64 {
	m := meanOf(src)
	var acc float64
	for _, v := range src {
		d := v - m
		acc += d * d
	}
	std := math.Sqrt(acc / float64(len(src)))
	if std < 1e-6 {
		std = 1e-6
	}
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = clamp01(0.5 + (v-m)*0.22/std)
	}
	return out
}

func reconstructNormals(height []float64, w, h int, targetRMS float64) (nx, ny, nz []float64) {
	n := w * h
	dx := make([]float64, n)
	dy := make([]float64, n)
	var energy float64
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := y*w + x
			dx[i] = sample(height, w, h, x+1, y) - sample(height, w, h, x-1, y)
			dy[i] = sample(height, w, h, x, y+1) - sample(height, w, h, x, y-1)
			energy += dx[i]*dx[i] + dy[i]*dy[i]
		}
	}
	rms := math.Sqrt(energy / float64(n))
	scale := targetRMS / math.Max(rms, 1e-9)
	nx = make([]float64, n)
	ny = make([]float64, n)
	nz = make([]float64, n)
	for i := 0; i < n; i++ {
		nx[i], ny[i], nz[i] = normalize3(-scale*dx[i], -scale*dy[i], 1)
	}
	return nx, ny, nz
}

func roughnessField(height []float64, w, h int) []float64 {
	// Uncoated newsprint is matte. Valleys and high-slope felt are
	// rougher; calendered highs keep a slightly tighter response.
	blur := gaussianWrap(height, w, h, 1.8)
	out := make([]float64, len(height))
	for i, hgt := range height {
		valley := clamp01((blur[i] - hgt) * 3.2)
		out[i] = clamp01(0.70 + 0.18*valley + 0.08*(1-hgt))
	}
	return out
}

func retintAlbedo(r, g, b []float64, w, h int) []float64 {
	n := w * h
	meanR, meanG, meanB := meanOf(r), meanOf(g), meanOf(b)
	out := make([]float64, n*3)
	for i := 0; i < n; i++ {
		out[i*3+0] = clamp8(baseR + (r[i]-meanR)*albedoGain)
		out[i*3+1] = clamp8(baseG + (g[i]-meanG)*albedoGain)
		out[i*3+2] = clamp8(baseB + (b[i]-meanB)*albedoGain)
	}
	return out
}

func shadePaper(albedo, height, nx, ny, nz, rough []float64, w, h int) []float64 {
	n := w * h
	lx, ly, lz := normalize3(-0.36, -0.58, 0.73)
	fx, fy, fz := normalize3(0.40, 0.16, 0.90)
	ao := microOcclusion(height, w, h)
	irr := make([]float64, n)
	spec := make([]float64, n)
	var meanI float64
	for i := 0; i < n; i++ {
		wrap := 0.08 + 0.22*rough[i]
		key := wrapDiffuse(nx[i]*lx+ny[i]*ly+nz[i]*lz, wrap)
		fill := wrapDiffuse(nx[i]*fx+ny[i]*fy+nz[i]*fz, wrap+0.06)
		irr[i] = (ambientWeight*ao[i] + keyWeight*key + fillWeight*fill)
		meanI += irr[i]
		hx, hy, hz := normalize3(lx, ly, lz+1)
		specPow := 7 + 20*(1-rough[i])
		specAmt := sheenPeak * (0.30 + 0.70*(1-rough[i])) * (0.50 + 0.50*height[i])
		spec[i] = math.Pow(clamp01(nx[i]*hx+ny[i]*hy+nz[i]*hz), specPow) * specAmt
	}
	meanI /= float64(n)

	out := make([]float64, n*3)
	for i := 0; i < n; i++ {
		shade := irr[i] / math.Max(meanI, 1e-6)
		out[i*3+0] = clamp8(albedo[i*3+0]*shade + spec[i]*252)
		out[i*3+1] = clamp8(albedo[i*3+1]*shade + spec[i]*248)
		out[i*3+2] = clamp8(albedo[i*3+2]*shade + spec[i]*236)
	}
	return out
}

func microOcclusion(height []float64, w, h int) []float64 {
	blur := gaussianWrap(height, w, h, aoRadius)
	out := make([]float64, len(height))
	for i, hgt := range height {
		out[i] = clamp01(1 - aoGain*clamp01((blur[i]-hgt)*4.2))
	}
	return out
}

func wrapDiffuse(ndotl, wrap float64) float64 {
	return clamp01((ndotl + wrap) / (1 + wrap))
}

func sample(src []float64, w, h, x, y int) float64 {
	x = (x%w + w) % w
	y = (y%h + h) % h
	return src[y*w+x]
}

func gaussianWrap(src []float64, w, h int, radius float64) []float64 {
	k := gaussKernel(radius)
	tmp := conv1D(src, w, h, k, true, true)
	return conv1D(tmp, w, h, k, false, true)
}

func gaussianRGBWrap(src []float64, w, h int, radius float64) []float64 {
	n := w * h
	ch := make([][]float64, 3)
	for c := 0; c < 3; c++ {
		plane := make([]float64, n)
		for i := 0; i < n; i++ {
			plane[i] = src[i*3+c]
		}
		ch[c] = gaussianWrap(plane, w, h, radius)
	}
	out := make([]float64, n*3)
	for i := 0; i < n; i++ {
		out[i*3+0] = ch[0][i]
		out[i*3+1] = ch[1][i]
		out[i*3+2] = ch[2][i]
	}
	return out
}

func gaussKernel(sigma float64) []float64 {
	if sigma <= 0.01 {
		return []float64{1}
	}
	radius := int(math.Ceil(sigma * 3))
	k := make([]float64, 2*radius+1)
	var sum float64
	for i := -radius; i <= radius; i++ {
		v := math.Exp(-float64(i*i) / (2 * sigma * sigma))
		k[i+radius] = v
		sum += v
	}
	for i := range k {
		k[i] /= sum
	}
	return k
}

func conv1D(src []float64, w, h int, k []float64, horizontal, wrap bool) []float64 {
	r := len(k) / 2
	dst := make([]float64, len(src))
	if horizontal {
		for y := 0; y < h; y++ {
			row := y * w
			for x := 0; x < w; x++ {
				var acc float64
				for i, kv := range k {
					xx := x + i - r
					if wrap {
						xx = (xx%w + w) % w
					} else if xx < 0 {
						xx = 0
					} else if xx >= w {
						xx = w - 1
					}
					acc += kv * src[row+xx]
				}
				dst[row+x] = acc
			}
		}
		return dst
	}
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			var acc float64
			for i, kv := range k {
				yy := y + i - r
				if wrap {
					yy = (yy%h + h) % h
				} else if yy < 0 {
					yy = 0
				} else if yy >= h {
					yy = h - 1
				}
				acc += kv * src[yy*w+x]
			}
			dst[y*w+x] = acc
		}
	}
	return dst
}

func resizeRGB(src []float64, sw, sh, dw, dh int) []float64 {
	if sw == dw && sh == dh {
		return src
	}
	out := make([]float64, dw*dh*3)
	xRatio := float64(sw-1) / float64(max(dw-1, 1))
	yRatio := float64(sh-1) / float64(max(dh-1, 1))
	for y := 0; y < dh; y++ {
		sy := yRatio * float64(y)
		y0 := int(sy)
		y1 := y0 + 1
		if y1 >= sh {
			y1 = sh - 1
		}
		ty := sy - float64(y0)
		for x := 0; x < dw; x++ {
			sx := xRatio * float64(x)
			x0 := int(sx)
			x1 := x0 + 1
			if x1 >= sw {
				x1 = sw - 1
			}
			tx := sx - float64(x0)
			for c := 0; c < 3; c++ {
				v00 := src[(y0*sw+x0)*3+c]
				v10 := src[(y0*sw+x1)*3+c]
				v01 := src[(y1*sw+x0)*3+c]
				v11 := src[(y1*sw+x1)*3+c]
				out[(y*dw+x)*3+c] = (v00*(1-tx)+v10*tx)*(1-ty) + (v01*(1-tx)+v11*tx)*ty
			}
		}
	}
	return out
}

func encodeJPEG(rgb []float64, w, h, quality int) ([]byte, error) {
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			i := (y*w + x) * 3
			img.SetRGBA(x, y, color.RGBA{
				R: uint8(math.Round(clamp8(rgb[i+0]))),
				G: uint8(math.Round(clamp8(rgb[i+1]))),
				B: uint8(math.Round(clamp8(rgb[i+2]))),
				A: 255,
			})
		}
	}
	var out bytes.Buffer
	if err := jpeg.Encode(&out, img, &jpeg.Options{Quality: quality}); err != nil {
		return nil, err
	}
	return out.Bytes(), nil
}

func meanOf(src []float64) float64 {
	var sum float64
	for _, v := range src {
		sum += v
	}
	return sum / float64(len(src))
}

func normalize3(x, y, z float64) (float64, float64, float64) {
	length := math.Sqrt(x*x + y*y + z*z)
	if length < 1e-12 {
		return 0, 0, 1
	}
	return x / length, y / length, z / length
}

func clamp8(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func clamp01(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 1 {
		return 1
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
