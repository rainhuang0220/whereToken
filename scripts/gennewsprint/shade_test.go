package main

import (
	"math"
	"testing"
)

func TestSRGBMidGrayRoundTrip(t *testing.T) {
	t.Parallel()
	lin := srgbToLinear(0.5)
	if math.Abs(lin-0.21404114048223255) > 1e-9 {
		t.Fatalf("linear=%g", lin)
	}
	back := linearToSRGB(lin)
	if math.Abs(back-0.5) > 1e-6 {
		t.Fatalf("srgb=%g", back)
	}
}

func TestRampNormalFacesDownhill(t *testing.T) {
	t.Parallel()
	w, h := 32, 8
	field := make([]float64, w*h)
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			field[y*w+x] = float64(x)
		}
	}
	n := normalAt(field, w, h, 16, 4, 1)
	if n.x >= 0 {
		t.Fatalf("n.x=%g want negative for a rising x ramp", n.x)
	}
	if n.z <= 0.5 {
		t.Fatalf("n.z=%g", n.z)
	}
	if math.Abs(lenVec(n)-1) > 1e-6 {
		t.Fatalf("normal length %g", lenVec(n))
	}
}

func TestConstantHeightNormalIsViewAligned(t *testing.T) {
	t.Parallel()
	w, h := 12, 9
	field := make([]float64, w*h)
	for i := range field {
		field[i] = 0.42
	}
	n := normalAt(field, w, h, 3, 4, normalScale)
	if math.Abs(n.x) > 1e-9 || math.Abs(n.y) > 1e-9 || math.Abs(n.z-1) > 1e-9 {
		t.Fatalf("normal=%+v", n)
	}
	lit := shadePaper([3]float64{0.94, 0.93, 0.91}, n, normalize(vec3{-0.32, -0.46, 0.82}), vec3{0, 0, 1}, 0.86)
	for _, c := range lit {
		if math.IsNaN(c) || math.IsInf(c, 0) || c < 0.7 || c > 1.2 {
			t.Fatalf("constant shade=%v", lit)
		}
	}
}

func TestKeyFacingSlopeIsBrighter(t *testing.T) {
	t.Parallel()
	light := normalize(vec3{-0.32, -0.46, 0.82})
	view := vec3{0, 0, 1}
	alb := [3]float64{0.94, 0.93, 0.91}
	toward := shadePaper(alb, normalize(light), light, view, 0.86)
	away := shadePaper(alb, normalize(vec3{-light.x, -light.y, light.z}), light, view, 0.86)
	if toward[0]+toward[1]+toward[2] <= away[0]+away[1]+away[2] {
		t.Fatalf("toward=%v away=%v", toward, away)
	}
}
