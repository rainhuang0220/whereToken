package main

import (
	"image"
	"math"
)

// Artistic scales for the Paper001 relative height proxy. The displacement
// JPEG is not a calibrated millimeter field. These numbers are screen-space
// art direction on top of one shared height.
const (
	sigmaBroad  = 42.0
	sigmaForm   = 7.5
	sigmaFiber  = 2.2
	broadAmp    = 0.07
	formAmp     = 0.48
	fiberAmp    = 0.82
	normalScale = 1.55
	roughBase   = 0.88
	roughAmp    = 0.03
	ambientGain = 0.88
	keyGain     = 0.12
	specGain    = 0.008
	chromaMix   = 0.10
	albedoForm  = 0.004
	paperSRGBR  = 252.0
	paperSRGBG  = 251.0
	paperSRGBB  = 247.0
)

type vec3 struct{ x, y, z float64 }

func bakeSheet(colorImg, dispImg image.Image) ([]float64, int, int, error) {
	b := colorImg.Bounds()
	if b != dispImg.Bounds() {
		return nil, 0, 0, errBounds(b, dispImg.Bounds())
	}
	sw, sh := b.Dx(), b.Dy()
	dw := productionWidth
	dh := int(math.Round(float64(sh) * float64(dw) / float64(sw)))
	if dh < 2 {
		dh = 2
	}
	_, r, g, bl := linearPlanes(colorImg)
	disp := heightPlane(dispImg)
	r = resizeWrap(r, sw, sh, dw, dh)
	g = resizeWrap(g, sw, sh, dw, dh)
	bl = resizeWrap(bl, sw, sh, dw, dh)
	disp = resizeWrap(disp, sw, sh, dw, dh)

	broad := gaussianWrap(disp, dw, dh, sigmaBroad)
	form := subPlane(gaussianWrap(disp, dw, dh, sigmaForm), broad)
	fiberDisp := highpassWrap(disp, dw, dh, sigmaFiber)
	luma := make([]float64, dw*dh)
	for i := range luma {
		luma[i] = 0.2126*r[i] + 0.7152*g[i] + 0.0722*bl[i]
	}
	fiberAlb := highpassWrap(luma, dw, dh, sigmaFiber)
	fiber := unitStd(addPlane(scalePlane(unitStd(fiberDisp), 0.82), scalePlane(unitStd(fiberAlb), 0.18)))
	formU := unitStd(form)
	height := addPlane(addPlane(scalePlane(unitStd(broad), broadAmp), scalePlane(formU, formAmp)), scalePlane(fiber, fiberAmp))
	height = gaussianWrap(height, dw, dh, 0.25)

	mean := meanPlane(r, g, bl)
	light := normalize(vec3{-0.32, -0.46, 0.82})
	view := vec3{0, 0, 1}
	base := [3]float64{srgbToLinear(paperSRGBR / 255), srgbToLinear(paperSRGBG / 255), srgbToLinear(paperSRGBB / 255)}
	out := make([]float64, dw*dh*3)
	for y := 0; y < dh; y++ {
		for x := 0; x < dw; x++ {
			i := y*dw + x
			n := normalAt(height, dw, dh, x, y, normalScale)
			rough := clamp01Range(roughBase+roughAmp*math.Tanh(formU[i]), 0.78, 0.94)
			alb := [3]float64{
				clamp01Range(base[0]+(r[i]-mean[0])*chromaMix+formU[i]*albedoForm, 0.90, 0.992),
				clamp01Range(base[1]+(g[i]-mean[1])*chromaMix+formU[i]*albedoForm, 0.90, 0.992),
				clamp01Range(base[2]+(bl[i]-mean[2])*chromaMix+formU[i]*albedoForm, 0.90, 0.992),
			}
			rgb := shadePaper(alb, n, light, view, rough)
			out[i*3+0] = linearToSRGB(rgb[0]) * 255
			out[i*3+1] = linearToSRGB(rgb[1]) * 255
			out[i*3+2] = linearToSRGB(rgb[2]) * 255
		}
	}
	return out, dw, dh, nil
}

func errBounds(a, b image.Rectangle) error {
	return &boundsError{a, b}
}

type boundsError struct{ a, b image.Rectangle }

func (e *boundsError) Error() string {
	return "color " + e.a.String() + " and displacement " + e.b.String() + " bounds differ"
}

func linearPlanes(img image.Image) (luma, r, g, b []float64) {
	bd := img.Bounds()
	n := bd.Dx() * bd.Dy()
	r = make([]float64, n)
	g = make([]float64, n)
	b = make([]float64, n)
	luma = make([]float64, n)
	i := 0
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			rr, gg, bb, _ := img.At(x, y).RGBA()
			r[i] = srgbToLinear(float64(rr>>8) / 255)
			g[i] = srgbToLinear(float64(gg>>8) / 255)
			b[i] = srgbToLinear(float64(bb>>8) / 255)
			luma[i] = 0.2126*r[i] + 0.7152*g[i] + 0.0722*b[i]
			i++
		}
	}
	return luma, r, g, b
}

func heightPlane(img image.Image) []float64 {
	bd := img.Bounds()
	out := make([]float64, bd.Dx()*bd.Dy())
	i := 0
	for y := bd.Min.Y; y < bd.Max.Y; y++ {
		for x := bd.Min.X; x < bd.Max.X; x++ {
			rr, _, _, _ := img.At(x, y).RGBA()
			out[i] = float64(rr>>8) / 255
			i++
		}
	}
	return out
}

func srgbToLinear(c float64) float64 {
	if c <= 0.04045 {
		return c / 12.92
	}
	return math.Pow((c+0.055)/1.055, 2.4)
}

func linearToSRGB(c float64) float64 {
	if c <= 0.0031308 {
		return 12.92 * c
	}
	return 1.055*math.Pow(c, 1/2.4) - 0.055
}

func normalAt(h []float64, w, height, x, y int, scale float64) vec3 {
	xm := (x - 1 + w) % w
	xp := (x + 1) % w
	ym := (y - 1 + height) % height
	yp := (y + 1) % height
	dx := (h[y*w+xp] - h[y*w+xm]) * 0.5
	dy := (h[yp*w+x] - h[ym*w+x]) * 0.5
	return normalize(vec3{-scale * dx, -scale * dy, 1})
}

func shadePaper(alb [3]float64, n, light, view vec3, rough float64) [3]float64 {
	diff := orenNayar(n, light, view, rough)
	spec := ggxSpec(n, light, view, rough) * specGain
	out := [3]float64{}
	lightColor := [3]float64{1, 0.985, 0.96}
	ambient := [3]float64{ambientGain, ambientGain * 0.995, ambientGain * 0.975}
	for c := 0; c < 3; c++ {
		v := alb[c]*(ambient[c]+keyGain*diff) + spec*lightColor[c]
		if v < 0 {
			v = 0
		}
		if math.IsNaN(v) || math.IsInf(v, 0) {
			v = alb[c]
		}
		out[c] = v
	}
	return out
}

func orenNayar(n, l, v vec3, rough float64) float64 {
	ndotl := clamp01(dot(n, l))
	ndotv := clamp01(dot(n, v))
	if ndotl <= 0 {
		return 0
	}
	sigma := 0.20 + 0.70*rough
	s2 := sigma * sigma
	A := 1 - 0.5*s2/(s2+0.33)
	B := 0.45 * s2 / (s2 + 0.09)
	lv := subVec(l, mulVec(n, dot(n, l)))
	vv := subVec(v, mulVec(n, dot(n, v)))
	cosPhi := 0.0
	if lenVec(lv) > 1e-8 && lenVec(vv) > 1e-8 {
		cosPhi = clamp11(dot(normalize(lv), normalize(vv)))
	}
	thetaI := math.Acos(ndotl)
	thetaR := math.Acos(ndotv)
	alpha := math.Max(thetaI, thetaR)
	beta := math.Min(thetaI, thetaR)
	return ndotl * (A + B*math.Max(0, cosPhi)*math.Sin(alpha)*math.Tan(beta))
}

func ggxSpec(n, l, v vec3, rough float64) float64 {
	ndotl := clamp01(dot(n, l))
	ndotv := clamp01(dot(n, v))
	if ndotl <= 1e-4 || ndotv <= 1e-4 {
		return 0
	}
	h := normalize(addVec(l, v))
	ndoth := clamp01(dot(n, h))
	vdoth := clamp01(dot(v, h))
	a := rough * rough
	a2 := a * a
	d := ndoth*ndoth*(a2-1) + 1
	D := a2 / (math.Pi * d * d)
	G := smithG(ndotv, a2) * smithG(ndotl, a2)
	f0 := 0.04
	F := f0 + (1-f0)*math.Pow(1-vdoth, 5)
	return D * G * F / (4 * ndotl * ndotv)
}

func smithG(ndotx, a2 float64) float64 {
	return (2 * ndotx) / (ndotx + math.Sqrt(a2+(1-a2)*ndotx*ndotx))
}

func gaussianWrap(src []float64, w, h int, sigma float64) []float64 {
	k := gaussKernel(sigma)
	tmp := convWrap(src, w, h, k, true)
	return convWrap(tmp, w, h, k, false)
}

func highpassWrap(src []float64, w, h int, sigma float64) []float64 {
	blurred := gaussianWrap(src, w, h, sigma)
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = v - blurred[i]
	}
	return out
}

func convWrap(src []float64, w, h int, k []float64, horizontal bool) []float64 {
	r := len(k) / 2
	dst := make([]float64, len(src))
	if horizontal {
		for y := 0; y < h; y++ {
			row := y * w
			for x := 0; x < w; x++ {
				var acc float64
				for i, kv := range k {
					xx := (x + i - r) % w
					if xx < 0 {
						xx += w
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
				yy := (y + i - r) % h
				if yy < 0 {
					yy += h
				}
				acc += kv * src[yy*w+x]
			}
			dst[y*w+x] = acc
		}
	}
	return dst
}

func resizeWrap(src []float64, sw, sh, dw, dh int) []float64 {
	if sw == dw && sh == dh {
		return append([]float64(nil), src...)
	}
	out := make([]float64, dw*dh)
	for y := 0; y < dh; y++ {
		fy := (float64(y)+0.5)*float64(sh)/float64(dh) - 0.5
		y0 := int(math.Floor(fy))
		ty := fy - float64(y0)
		y0 = wrapIndex(y0, sh)
		y1 := wrapIndex(y0+1, sh)
		for x := 0; x < dw; x++ {
			fx := (float64(x)+0.5)*float64(sw)/float64(dw) - 0.5
			x0 := int(math.Floor(fx))
			tx := fx - float64(x0)
			x0 = wrapIndex(x0, sw)
			x1 := wrapIndex(x0+1, sw)
			v00 := src[y0*sw+x0]
			v10 := src[y0*sw+x1]
			v01 := src[y1*sw+x0]
			v11 := src[y1*sw+x1]
			out[y*dw+x] = (v00*(1-tx)+v10*tx)*(1-ty) + (v01*(1-tx)+v11*tx)*ty
		}
	}
	return out
}

func wrapIndex(i, n int) int {
	if n <= 0 {
		return 0
	}
	i %= n
	if i < 0 {
		i += n
	}
	return i
}

func unitStd(src []float64) []float64 {
	if len(src) == 0 {
		return nil
	}
	mean := 0.0
	for _, v := range src {
		mean += v
	}
	mean /= float64(len(src))
	acc := 0.0
	for _, v := range src {
		d := v - mean
		acc += d * d
	}
	std := math.Sqrt(acc / float64(len(src)))
	if std < 1e-8 {
		std = 1
	}
	out := make([]float64, len(src))
	for i, v := range src {
		out[i] = (v - mean) / std
	}
	return out
}

func addPlane(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] + b[i]
	}
	return out
}

func subPlane(a, b []float64) []float64 {
	out := make([]float64, len(a))
	for i := range a {
		out[i] = a[i] - b[i]
	}
	return out
}

func scalePlane(a []float64, s float64) []float64 {
	out := make([]float64, len(a))
	for i, v := range a {
		out[i] = v * s
	}
	return out
}

func meanPlane(r, g, b []float64) [3]float64 {
	var acc [3]float64
	for i := range r {
		acc[0] += r[i]
		acc[1] += g[i]
		acc[2] += b[i]
	}
	n := float64(len(r))
	return [3]float64{acc[0] / n, acc[1] / n, acc[2] / n}
}

func dot(a, b vec3) float64 { return a.x*b.x + a.y*b.y + a.z*b.z }

func addVec(a, b vec3) vec3 { return vec3{a.x + b.x, a.y + b.y, a.z + b.z} }

func subVec(a, b vec3) vec3 { return vec3{a.x - b.x, a.y - b.y, a.z - b.z} }

func mulVec(a vec3, s float64) vec3 { return vec3{a.x * s, a.y * s, a.z * s} }

func lenVec(a vec3) float64 { return math.Hypot(a.x, math.Hypot(a.y, a.z)) }

func normalize(a vec3) vec3 {
	l := lenVec(a)
	if l < 1e-12 {
		return vec3{0, 0, 1}
	}
	return mulVec(a, 1/l)
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

func clamp11(v float64) float64 {
	if v < -1 {
		return -1
	}
	if v > 1 {
		return 1
	}
	return v
}

func clamp01Range(v, lo, hi float64) float64 {
	if v < lo {
		return lo
	}
	if v > hi {
		return hi
	}
	return v
}

func lumaStats(rgb []float64, w, h int) (mean, std, edge float64) {
	n := w * h
	if n == 0 {
		return 0, 0, 0
	}
	acc := 0.0
	pix := make([]float64, n)
	for i := 0; i < n; i++ {
		pix[i] = 0.2126*rgb[i*3] + 0.7152*rgb[i*3+1] + 0.0722*rgb[i*3+2]
		acc += pix[i]
	}
	mean = acc / float64(n)
	var dev float64
	for _, v := range pix {
		d := v - mean
		dev += d * d
	}
	std = math.Sqrt(dev / float64(n))
	var eacc float64
	var count float64
	for y := 0; y < h; y++ {
		eacc += math.Abs(pix[y*w] - pix[y*w+w-1])
		count++
	}
	for x := 0; x < w; x++ {
		eacc += math.Abs(pix[x] - pix[(h-1)*w+x])
		count++
	}
	edge = eacc / count
	return mean, std, edge
}
