package main

import "math"

func fade(t float64) float64 {
	return t * t * t * (t*(t*6-15) + 10)
}

func lattice(ix, iy int32, seed uint64) float64 {
	n := seed
	n ^= uint64(uint32(ix)) * 0x9e3779b97f4a7c15
	n ^= uint64(uint32(iy)) * 0xc2b2ae3d27d4eb4f
	n ^= n >> 30
	n *= 0xbf58476d1ce4e5b9
	n ^= n >> 27
	n *= 0x94d049bb133111eb
	n ^= n >> 31
	return float64(n>>11)/(1<<53)*2 - 1
}

func valueNoise(x, y float64, seed uint64) float64 {
	x0 := math.Floor(x)
	y0 := math.Floor(y)
	ix := int32(x0)
	iy := int32(y0)
	u := fade(x - x0)
	v := fade(y - y0)
	n00 := lattice(ix, iy, seed)
	n10 := lattice(ix+1, iy, seed)
	n01 := lattice(ix, iy+1, seed)
	n11 := lattice(ix+1, iy+1, seed)
	nx0 := n00 + u*(n10-n00)
	nx1 := n01 + u*(n11-n01)
	return nx0 + v*(nx1-nx0)
}

func fbm2(x, y float64, octaves int, seed uint64) float64 {
	var sum, amp, norm float64
	amp = 1
	freq := 1.0
	for i := 0; i < octaves; i++ {
		sum += amp * valueNoise(x*freq, y*freq, seed+uint64(i)*19)
		norm += amp
		amp *= 0.5
		freq *= 2.07
	}
	if norm == 0 {
		return 0
	}
	return sum / norm
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

func smoothstep(e0, e1, x float64) float64 {
	if e0 == e1 {
		return clamp01(x - e0)
	}
	t := clamp01((x - e0) / (e1 - e0))
	return t * t * (3 - 2*t)
}

func gate(s, lo, hi float64) float64 {
	return smoothstep(lo, lo+0.14, s) * (1 - smoothstep(hi-0.14, hi, s))
}
