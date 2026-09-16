package publicprofile

import (
	"fmt"
	"math"
)

const absoluteDailyTokenCap int64 = 1_000_000_000

// magnitudeScale is a continuous presentation scale with a stable absolute
// token/day cap. Snapshot levels remain available for schema compatibility.
type magnitudeScale struct{}

func newMagnitudeScale(_ []int64, _ []string) magnitudeScale {
	return magnitudeScale{}
}

func (magnitudeScale) intensity(value int64) float64 {
	if value <= 0 {
		return 0
	}
	ratio := float64(value) / float64(absoluteDailyTokenCap)
	if ratio > 1 {
		ratio = 1
	}
	return math.Sqrt(ratio)
}

type OKLCH struct {
	L float64
	C float64
	H float64
}

type HeatRamp struct {
	Low  OKLCH
	Mid  OKLCH
	High OKLCH
	Max  OKLCH
}

const (
	heatMidPosition  = 0.25
	heatHighPosition = 0.60
)

func heatColor(ramp HeatRamp, intensity float64) string {
	return oklchHex(gamutMapToSRGB(ramp.colorAt(intensity)))
}

func (ramp HeatRamp) colorAt(intensity float64) OKLCH {
	if intensity < 0 {
		intensity = 0
	}
	if intensity > 1 {
		intensity = 1
	}
	switch {
	case intensity <= heatMidPosition:
		return interpolateOKLCH(ramp.Low, ramp.Mid, intensity/heatMidPosition)
	case intensity <= heatHighPosition:
		return interpolateOKLCH(ramp.Mid, ramp.High, (intensity-heatMidPosition)/(heatHighPosition-heatMidPosition))
	default:
		return interpolateOKLCH(ramp.High, ramp.Max, (intensity-heatHighPosition)/(1-heatHighPosition))
	}
}

func interpolateOKLCH(from, to OKLCH, t float64) OKLCH {
	return OKLCH{
		L: from.L + (to.L-from.L)*t,
		C: from.C + (to.C-from.C)*t,
		H: from.H + (to.H-from.H)*t,
	}
}

func gamutMapToSRGB(color OKLCH) OKLCH {
	if inSRGBGamut(color) {
		return color
	}
	low, high := 0.0, color.C
	for i := 0; i < 24; i++ {
		mid := (low + high) / 2
		candidate := color
		candidate.C = mid
		if inSRGBGamut(candidate) {
			low = mid
		} else {
			high = mid
		}
	}
	color.C = low
	return color
}

func inSRGBGamut(color OKLCH) bool {
	for _, component := range oklchLinearSRGB(color) {
		if component < 0 || component > 1 {
			return false
		}
	}
	return true
}

func oklchHex(color OKLCH) string {
	linear := oklchLinearSRGB(color)
	return fmt.Sprintf("#%02X%02X%02X", srgbByte(linear[0]), srgbByte(linear[1]), srgbByte(linear[2]))
}

func oklchLinearSRGB(color OKLCH) [3]float64 {
	h := color.H * math.Pi / 180
	a := color.C * math.Cos(h)
	b := color.C * math.Sin(h)

	lRoot := color.L + 0.3963377774*a + 0.2158037573*b
	mRoot := color.L - 0.1055613458*a - 0.0638541728*b
	sRoot := color.L - 0.0894841775*a - 1.291485548*b
	l := lRoot * lRoot * lRoot
	m := mRoot * mRoot * mRoot
	s := sRoot * sRoot * sRoot

	red := 4.0767416621*l - 3.3077115913*m + 0.2309699292*s
	green := -1.2684380046*l + 2.6097574011*m - 0.3413193965*s
	blue := -0.0041960863*l - 0.7034186147*m + 1.707614701*s
	return [3]float64{red, green, blue}
}

func srgbByte(linear float64) int {
	var encoded float64
	if linear <= 0.0031308 {
		encoded = 12.92 * linear
	} else {
		encoded = 1.055*math.Pow(linear, 1/2.4) - 0.055
	}
	if encoded < 0 {
		encoded = 0
	}
	if encoded > 1 {
		encoded = 1
	}
	return int(math.Round(encoded * 255))
}
