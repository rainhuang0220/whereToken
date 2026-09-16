package main

import (
	"fmt"
	"io"
	"math"
	"runtime"
	"sync"
)

type fourierMode struct {
	kx, ky, phase, weight float64
}

type featureKind int

const (
	featHill featureKind = iota
	featPucker
	featSaddle
	featBuckle
	featGrain
)

type feature struct {
	kind                         featureKind
	cx, cy                       float64
	rx, ry                       float64
	cos, sin                     float64
	amp, warp                    float64
	p0x, p0y, p1x, p1y, p2x, p2y float64
	width, cycles, phase         float64
	grainSeed                    uint64
	xmin, xmax, ymin, ymax       float64
	envCx, envCy, envR           float64
	useEnv                       bool
}

type paperModel struct {
	w, h                          int
	sx, sy                        float64
	seedAmp, seedAmp2, seedScale  uint64
	seedWarpX, seedWarpY          uint64
	seedForm, seedWave            uint64
	seedCoarse, seedMid, seedFine uint64
	modes                         []fourierMode
	feats                         []feature
}

type regionStat struct {
	row, col                int
	heightRMS, slopeRMS     float64
	variance, dominantScale float64
}

func (s sheetScale) x(ref float64) float64 { return ref * s.sx }
func (s sheetScale) y(ref float64) float64 { return ref * s.sy }
func (s sheetScale) r(ref float64) float64 { return ref * s.sx }

type sheetScale struct{ sx, sy float64 }

func buildModel(width, height int, seed uint64) *paperModel {
	rng := splitMix64{state: seed}
	sc := sheetScale{
		sx: float64(width) / float64(displayWidth),
		sy: float64(height) / float64(displayHeight),
	}
	m := &paperModel{
		w:          width,
		h:          height,
		sx:         sc.sx,
		sy:         sc.sy,
		seedAmp:    rng.next(),
		seedAmp2:   rng.next(),
		seedScale:  rng.next(),
		seedWarpX:  rng.next(),
		seedWarpY:  rng.next(),
		seedForm:   rng.next(),
		seedWave:   rng.next(),
		seedCoarse: rng.next(),
		seedMid:    rng.next(),
		seedFine:   rng.next(),
	}
	m.modes = makeFourier(&rng, sc, 7)
	m.feats = placeFeatures(&rng, sc, width, height)
	return m
}

func makeFourier(rng *splitMix64, sc sheetScale, count int) []fourierMode {
	modes := make([]fourierMode, 0, count)
	for len(modes) < count {
		wavelength := sc.r(52 + 128*rng.unit())
		if wavelength < 4 {
			continue
		}
		angle := 2 * math.Pi * rng.unit()
		k := 2 * math.Pi / wavelength
		modes = append(modes, fourierMode{
			kx:     k * math.Cos(angle),
			ky:     k * math.Sin(angle),
			phase:  2 * math.Pi * rng.unit(),
			weight: 0.55 + 0.55*rng.unit(),
		})
	}
	return modes
}

func placeFeatures(rng *splitMix64, sc sheetScale, width, height int) []feature {
	feats := make([]feature, 0, 64)
	place := func(n int, minDist, minR, maxR float64, build func(cx, cy, rx, ry, rot float64) feature) {
		placed := 0
		for attempt := 0; attempt < n*40 && placed < n; attempt++ {
			cx := 8 + rng.unit()*math.Max(1, float64(width)-16)
			cy := 8 + rng.unit()*math.Max(1, float64(height)-16)
			if tooClose(feats, cx, cy, sc.r(minDist)) {
				continue
			}
			rx := sc.r(minR + (maxR-minR)*rng.unit())
			ry := rx * (0.58 + 0.84*rng.unit())
			rot := 2 * math.Pi * rng.unit()
			feats = append(feats, build(cx, cy, rx, ry, rot))
			placed++
		}
	}

	place(8, 200, 40, 150, func(cx, cy, rx, ry, rot float64) feature {
		sign := 1.0
		if rng.unit() < 0.22 {
			sign = -1
		}
		f := feature{
			kind: featHill,
			cx:   cx, cy: cy, rx: rx, ry: ry,
			cos: math.Cos(rot), sin: math.Sin(rot),
			amp:  sign * (0.16 + 0.28*rng.unit()),
			warp: sc.r(6 + 12*rng.unit()),
		}
		f.setBounds(3.2)
		return f
	})
	place(8, 150, 24, 90, func(cx, cy, rx, ry, rot float64) feature {
		f := feature{
			kind: featPucker,
			cx:   cx, cy: cy, rx: rx, ry: ry,
			cos: math.Cos(rot), sin: math.Sin(rot),
			amp:  0.18 + 0.22*rng.unit(),
			warp: sc.r(5 + 8*rng.unit()),
		}
		if rng.unit() < 0.3 {
			f.amp = -f.amp
		}
		f.setBounds(3.6)
		return f
	})
	place(7, 190, 80, 230, func(cx, cy, rx, ry, rot float64) feature {
		f := feature{
			kind: featSaddle,
			cx:   cx, cy: cy, rx: rx, ry: ry,
			cos: math.Cos(rot), sin: math.Sin(rot),
			amp:  0.18 + 0.28*rng.unit(),
			warp: sc.r(8 + 10*rng.unit()),
		}
		f.setBounds(3.0)
		return f
	})
	place(8, 150, 36, 150, func(cx, cy, length, _, rot float64) feature {
		width := sc.r(12 + 24*rng.unit())
		amp := 0.16 + 0.22*rng.unit()
		if rng.unit() < 0.18 {
			amp = 0.36 + 0.20*rng.unit()
		}
		if rng.unit() < 0.4 {
			amp = -amp
		}
		half := length * 0.5
		cos, sin := math.Cos(rot), math.Sin(rot)
		bend := (rng.unit()*2 - 1) * length * (0.12 + 0.28*rng.unit())
		p0x, p0y := cx-cos*half, cy-sin*half
		p2x, p2y := cx+cos*half, cy+sin*half
		p1x := cx - sin*bend
		p1y := cy + cos*bend
		f := feature{
			kind: featBuckle,
			p0x:  p0x, p0y: p0y, p1x: p1x, p1y: p1y, p2x: p2x, p2y: p2y,
			width:  width,
			amp:    amp,
			cycles: 0.35 + 0.7*rng.unit(),
			phase:  2 * math.Pi * rng.unit(),
		}
		pad := width * 4
		f.xmin = min4(p0x, p1x, p2x) - pad
		f.xmax = max4(p0x, p1x, p2x) + pad
		f.ymin = min4(p0y, p1y, p2y) - pad
		f.ymax = max4(p0y, p1y, p2y) + pad
		return f
	})

	for i := 0; i < 2; i++ {
		cx := 40 + rng.unit()*math.Max(1, float64(width)-80)
		cy := 40 + rng.unit()*math.Max(1, float64(height)-80)
		envR := sc.r(90 + 200*rng.unit())
		if tooClose(feats, cx, cy, envR*0.7) && i < 8 {
			cx = 40 + rng.unit()*math.Max(1, float64(width)-80)
			cy = 40 + rng.unit()*math.Max(1, float64(height)-80)
		}
		nLocal := 3 + int(rng.next()%2)
		for j := 0; j < nLocal; j++ {
			ang := 2 * math.Pi * rng.unit()
			rad := envR * (0.15 + 0.55*rng.unit())
			px := cx + math.Cos(ang)*rad
			py := cy + math.Sin(ang)*rad
			rx := sc.r(14 + 36*rng.unit())
			ry := rx * (0.6 + 0.7*rng.unit())
			rot := 2 * math.Pi * rng.unit()
			f := feature{
				kind: featPucker,
				cx:   px, cy: py, rx: rx, ry: ry,
				cos: math.Cos(rot), sin: math.Sin(rot),
				amp:    (0.08 + 0.12*rng.unit()) * signUnit(rng),
				warp:   sc.r(3),
				useEnv: true, envCx: cx, envCy: cy, envR: envR,
			}
			f.setBounds(3.4)
			feats = append(feats, f)
		}
		rot := 2 * math.Pi * rng.unit()
		saddle := feature{
			kind: featSaddle,
			cx:   cx, cy: cy,
			rx: sc.r(50 + 70*rng.unit()), ry: sc.r(40 + 80*rng.unit()),
			cos: math.Cos(rot), sin: math.Sin(rot),
			amp:    0.10 + 0.14*rng.unit(),
			useEnv: true, envCx: cx, envCy: cy, envR: envR,
		}
		saddle.setBounds(3.0)
		feats = append(feats, saddle)
		for j := 0; j < 2; j++ {
			ang := 2 * math.Pi * rng.unit()
			length := sc.r(28 + 70*rng.unit())
			width := sc.r(8 + 18*rng.unit())
			cos, sin := math.Cos(ang), math.Sin(ang)
			bx := cx + (rng.unit()*2-1)*envR*0.25
			by := cy + (rng.unit()*2-1)*envR*0.25
			half := length * 0.5
			bend := (rng.unit()*2 - 1) * length * 0.2
			p0x, p0y := bx-cos*half, by-sin*half
			p2x, p2y := bx+cos*half, by+sin*half
			buckle := feature{
				kind: featBuckle,
				p0x:  p0x, p0y: p0y,
				p1x: bx - sin*bend, p1y: by + cos*bend,
				p2x: p2x, p2y: p2y,
				width:  width,
				amp:    (0.10 + 0.16*rng.unit()) * signUnit(rng),
				cycles: 0.4 + 0.5*rng.unit(),
				phase:  2 * math.Pi * rng.unit(),
				useEnv: true, envCx: cx, envCy: cy, envR: envR,
			}
			pad := width * 4
			buckle.xmin = min4(p0x, buckle.p1x, p2x) - pad
			buckle.xmax = max4(p0x, buckle.p1x, p2x) + pad
			buckle.ymin = min4(p0y, buckle.p1y, p2y) - pad
			buckle.ymax = max4(p0y, buckle.p1y, p2y) + pad
			feats = append(feats, buckle)
		}
		grain := feature{
			kind: featGrain,
			cx:   cx, cy: cy,
			amp:       0.06 + 0.05*rng.unit(),
			grainSeed: rng.next(),
			rx:        envR, ry: envR,
			useEnv: true, envCx: cx, envCy: cy, envR: envR,
			xmin: cx - envR, xmax: cx + envR, ymin: cy - envR, ymax: cy + envR,
		}
		feats = append(feats, grain)
	}
	return feats
}

func (f *feature) setBounds(spread float64) {
	reach := spread*math.Max(f.rx, f.ry) + f.warp*2
	f.xmin = f.cx - reach
	f.xmax = f.cx + reach
	f.ymin = f.cy - reach
	f.ymax = f.cy + reach
}

func tooClose(feats []feature, x, y, min float64) bool {
	min2 := min * min
	for i := range feats {
		if feats[i].useEnv {
			continue
		}
		cx, cy := feats[i].cx, feats[i].cy
		if feats[i].kind == featBuckle {
			cx = (feats[i].p0x + feats[i].p2x) * 0.5
			cy = (feats[i].p0y + feats[i].p2y) * 0.5
		}
		dx, dy := x-cx, y-cy
		if dx*dx+dy*dy < min2 {
			return true
		}
	}
	return false
}

func signUnit(rng *splitMix64) float64 {
	if rng.unit() < 0.5 {
		return -1
	}
	return 1
}

func (m *paperModel) amplitude(x, y float64) float64 {
	n := fbm2(x/(720*m.sx), y/(800*m.sy), 3, m.seedAmp)
	n += 0.32 * fbm2(x/(390*m.sx), y/(430*m.sy), 2, m.seedAmp2)
	t := clamp01(0.5 + 0.52*n)
	t = t * t * (3 - 2*t)
	t = math.Pow(t, 1.55)
	return 0.08 + 1.00*t
}

func (m *paperModel) scaleField(x, y float64) float64 {
	n := fbm2(x/(660*m.sx), y/(720*m.sy), 2, m.seedScale)
	t := clamp01(0.5 + 0.55*n)
	return t * t * (3 - 2*t)
}

func (m *paperModel) sample(x, y float64) float64 {
	A := m.amplitude(x, y)
	S := m.scaleField(x, y)
	wx := x + 24*m.sx*fbm2(x/(118*m.sx), y/(118*m.sy), 2, m.seedWarpX)
	wy := y + 24*m.sy*fbm2(x/(118*m.sx)+31, y/(118*m.sy), 2, m.seedWarpY)

	h := 0.05 * fbm2(x/(680*m.sx), y/(760*m.sy), 2, m.seedForm)
	h += 0.12 * (0.25 + 0.75*A) * fbm2(wx/(300*m.sx), wy/(330*m.sy), 1, m.seedWave)

	coarse := fbm2(wx/(190*m.sx), wy/(210*m.sy), 1, m.seedCoarse)
	mid := fbm2(wx/(100*m.sx), wy/(112*m.sy), 1, m.seedMid)
	fine := fbm2(wx/(52*m.sx), wy/(58*m.sy), 1, m.seedFine)
	h += A * (0.55*gate(S, 0.06, 0.52)*coarse + 0.42*gate(S, 0.30, 0.80)*mid + 0.18*gate(S, 0.58, 1.08)*fine)

	var fsum float64
	for _, mode := range m.modes {
		fsum += mode.weight * math.Sin(mode.kx*wx+mode.ky*wy+mode.phase)
	}
	h += (0.10 + 0.16*A) * fsum / float64(len(m.modes))

	for i := range m.feats {
		h += m.feats[i].at(x, y)
	}
	return h
}

func (f *feature) at(x, y float64) float64 {
	if x < f.xmin || x > f.xmax || y < f.ymin || y > f.ymax {
		return 0
	}
	v := f.raw(x, y)
	if f.useEnv {
		dx, dy := x-f.envCx, y-f.envCy
		q := (dx*dx + dy*dy) / (f.envR * f.envR)
		if q >= 1 {
			return 0
		}
		v *= (1 - q) * (1 - q)
	}
	return v
}

func (f *feature) raw(x, y float64) float64 {
	switch f.kind {
	case featGrain:
		return f.amp * valueNoise(x/68, y/74, f.grainSeed)
	case featBuckle:
		return buckleAt(f, x, y)
	}
	dx := x - f.cx
	dy := y - f.cy
	if f.warp != 0 {
		dx += f.warp * valueNoise(x*0.027, y*0.027, 0x51ed)
		dy += f.warp * valueNoise(x*0.027+17, y*0.027, 0x51ee)
	}
	u := dx*f.cos + dy*f.sin
	v := -dx*f.sin + dy*f.cos
	qx := u / f.rx
	qy := v / f.ry
	q := qx*qx + qy*qy
	if q > 12 {
		return 0
	}
	switch f.kind {
	case featHill:
		return f.amp * math.Exp(-q)
	case featPucker:
		inner := math.Exp(-q)
		outer := math.Exp(-q / 2.56)
		return f.amp * (inner - 0.62*outer)
	case featSaddle:
		env := math.Exp(-q)
		return f.amp * env * (qx*qx - qy*qy)
	}
	return 0
}

func buckleAt(f *feature, x, y float64) float64 {
	bestT := 0.0
	bestD := 1e18
	const steps = 18
	for i := 0; i <= steps; i++ {
		t := float64(i) / float64(steps)
		px, py := quadBez(f, t)
		dx, dy := x-px, y-py
		d := dx*dx + dy*dy
		if d < bestD {
			bestD = d
			bestT = t
		}
	}
	lo := math.Max(0, bestT-1/float64(steps))
	hi := math.Min(1, bestT+1/float64(steps))
	for i := 0; i <= 6; i++ {
		t := lo + (hi-lo)*float64(i)/6
		px, py := quadBez(f, t)
		dx, dy := x-px, y-py
		d := dx*dx + dy*dy
		if d < bestD {
			bestD = d
			bestT = t
		}
	}
	perp := math.Sqrt(bestD)
	if perp > f.width*3.2 {
		return 0
	}
	taper := smoothstep(0, 0.16, bestT) * (1 - smoothstep(0.84, 1, bestT))
	ridge := math.Exp(-(perp / f.width) * (perp / f.width))
	mod := 0.72 + 0.28*math.Sin(2*math.Pi*bestT*f.cycles+f.phase)
	return f.amp * taper * ridge * mod
}

func quadBez(f *feature, t float64) (float64, float64) {
	mt := 1 - t
	a, b, c := mt*mt, 2*mt*t, t*t
	return a*f.p0x + b*f.p1x + c*f.p2x, a*f.p0y + b*f.p1y + c*f.p2y
}

func heightField(width, height int, seed uint64) []float64 {
	model := buildModel(width, height, seed)
	out := make([]float64, width*height)
	workers := runtime.GOMAXPROCS(0)
	if workers < 1 {
		workers = 1
	}
	var wg sync.WaitGroup
	rows := make(chan int, workers*2)
	wg.Add(workers)
	for i := 0; i < workers; i++ {
		go func() {
			defer wg.Done()
			for y := range rows {
				base := y * width
				fy := float64(y)
				for x := 0; x < width; x++ {
					out[base+x] = model.sample(float64(x), fy)
				}
			}
		}()
	}
	for y := 0; y < height; y++ {
		rows <- y
	}
	close(rows)
	wg.Wait()
	return out
}

func regionStats(heights []float64, width, height, gw, gh int) []regionStat {
	if gw < 1 {
		gw = 1
	}
	if gh < 1 {
		gh = 1
	}
	stats := make([]regionStat, 0, gw*gh)
	cw := width / gw
	ch := height / gh
	for row := 0; row < gh; row++ {
		for col := 0; col < gw; col++ {
			x0, y0 := col*cw, row*ch
			x1, y1 := x0+cw, y0+ch
			if col == gw-1 {
				x1 = width
			}
			if row == gh-1 {
				y1 = height
			}
			stats = append(stats, summarizeRegion(heights, width, height, col, row, x0, y0, x1, y1))
		}
	}
	return stats
}

func summarizeRegion(heights []float64, width, height, col, row, x0, y0, x1, y1 int) regionStat {
	var sum, sum2, slope2 float64
	var n, crossings int
	var prevSign int
	havePrev := false
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			h := heights[y*width+x]
			sum += h
			sum2 += h * h
			n++
			if x+1 < width && x-1 >= 0 {
				dx := heights[y*width+(x+1)] - heights[y*width+(x-1)]
				slope2 += dx * dx
			}
			if y+1 < height && y-1 >= 0 {
				dy := heights[(y+1)*width+x] - heights[(y-1)*width+x]
				slope2 += dy * dy
			}
		}
	}
	mean := sum / math.Max(float64(n), 1)
	for y := y0; y < y1; y++ {
		for x := x0; x < x1; x++ {
			h := heights[y*width+x] - mean
			sign := 1
			if h < 0 {
				sign = -1
			}
			if havePrev && sign != prevSign && math.Abs(h) > 1e-6 {
				crossings++
			}
			prevSign = sign
			havePrev = true
		}
	}
	variance := sum2/math.Max(float64(n), 1) - mean*mean
	if variance < 0 {
		variance = 0
	}
	scale := 0.0
	if crossings > 0 {
		scale = 2 * float64(n) / float64(crossings)
	}
	return regionStat{
		row:           row,
		col:           col,
		heightRMS:     math.Sqrt(variance),
		slopeRMS:      math.Sqrt(slope2 / math.Max(float64(n), 1)),
		variance:      variance,
		dominantScale: scale,
	}
}

func printRegionDiagnostics(w io.Writer, width, height int, stats []regionStat) {
	fmt.Fprintf(w, "newsprint 4x4 region diagnostics (%dx%d)\n", width, height)
	fmt.Fprintf(w, " r c   hRMS     sRMS      var    scale\n")
	minH, maxH := math.MaxFloat64, 0.0
	minS, maxS := math.MaxFloat64, 0.0
	for _, s := range stats {
		fmt.Fprintf(w, "%2d %d  %7.4f  %7.4f  %7.4f  %6.1f\n", s.row, s.col, s.heightRMS, s.slopeRMS, s.variance, s.dominantScale)
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
	}
	fmt.Fprintf(w, "hRMS range %.4f .. %.4f  ratio %.2f\n", minH, maxH, maxH/math.Max(minH, 1e-9))
	fmt.Fprintf(w, "sRMS range %.4f .. %.4f  ratio %.2f\n", minS, maxS, maxS/math.Max(minS, 1e-9))
}

func min4(a, b, c float64) float64 {
	return math.Min(a, math.Min(b, c))
}

func max4(a, b, c float64) float64 {
	return math.Max(a, math.Max(b, c))
}
