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

const (
	dispBlurRadius    = 6.2
	albedoBlurRadius  = 4.4
	fiberDispWeight   = 0.72
	fiberAlbedoWeight = 0.28
	mixAlbedoWeight   = 0.62
	mixDispWeight     = 0.38
	fiberUnitStd      = 12.0
	fiberGain         = 0.44
	softenRadius      = 0.35
	baseR             = 247.0
	baseG             = 246.0
	baseB             = 241.0
	jpegQuality       = 86
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
	rgb, dw, dh, err := bakeSheet(colorImg, dispImg)
	if err != nil {
		return nil, image.Point{}, err
	}
	quality := jpegQuality
	var payload []byte
	for {
		payload, err = encodeJPEG(rgb, dw, dh, quality)
		if err != nil {
			return nil, image.Point{}, err
		}
		if len(payload) <= maxSurfaceBytes || quality <= 74 {
			break
		}
		quality -= 4
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

func highpass(src []float64, w, h int, radius float64) []float64 {
	blurred := gaussian(src, w, h, radius)
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = v - blurred[i]
	}
	return out
}

func unitFiber(src []float64, n int) []float64 {
	var mean float64
	for _, v := range src {
		mean += v
	}
	mean /= float64(n)
	var acc float64
	for _, v := range src {
		d := v - mean
		acc += d * d
	}
	std := math.Sqrt(acc / float64(n))
	if std < 1e-6 {
		std = 1e-6
	}
	scale := fiberUnitStd / std
	out := make([]float64, n)
	for i, v := range src {
		out[i] = (v - mean) * scale
	}
	return out
}

func weightedSum(a, b []float64, wa, wb float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = wa*a[i] + wb*b[i]
	}
	return out
}

func colorize(fiber, _, _, _ []float64, w, h int) []float64 {
	out := make([]float64, w*h*3)
	for i, v := range fiber {
		s := v * fiberGain
		out[i*3+0] = clamp8(baseR + s)
		out[i*3+1] = clamp8(baseG + s)
		out[i*3+2] = clamp8(baseB + s)
	}
	return out
}

func gaussian(src []float64, w, h int, radius float64) []float64 {
	k := gaussKernel(radius)
	tmp := conv1D(src, w, h, k, true)
	return conv1D(tmp, w, h, k, false)
}

func gaussianRGB(src []float64, w, h int, radius float64) []float64 {
	n := w * h
	ch := make([][]float64, 3)
	for c := 0; c < 3; c++ {
		plane := make([]float64, n)
		for i := 0; i < n; i++ {
			plane[i] = src[i*3+c]
		}
		ch[c] = gaussian(plane, w, h, radius)
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

func conv1D(src []float64, w, h int, k []float64, horizontal bool) []float64 {
	r := len(k) / 2
	dst := make([]float64, len(src))
	if horizontal {
		for y := 0; y < h; y++ {
			row := y * w
			for x := 0; x < w; x++ {
				var acc float64
				for i, kv := range k {
					xx := x + i - r
					if xx < 0 {
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
				if yy < 0 {
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

func clamp8(v float64) float64 {
	if v < 0 {
		return 0
	}
	if v > 255 {
		return 255
	}
	return v
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
