package publicprofile

import (
	"math"
	"strconv"
	"strings"

	"github.com/rainhuang0220/whereToken/internal/metric"
)

func availableInt(n int64) Component {
	v := n
	return Component{Value: &v, Display: FormatCompact(n), Status: StatusAvailable}
}

func unavailableComponent() Component {
	return Component{Value: nil, Display: emDash, Status: StatusUnavailable}
}

func availableCount(n int64) Count {
	v := n
	return Count{Value: &v, Display: strconv.FormatInt(n, 10), Status: StatusAvailable}
}

func unavailableCount() Count {
	return Count{Value: nil, Display: emDash, Status: StatusUnavailable}
}

func hitRateOf(miss, cacheRead, cacheCreate int64, unavailable bool) HitRate {
	if unavailable {
		return HitRate{Value: nil, Display: emDash}
	}
	pct, ok := metric.HitRate(miss, cacheRead, cacheCreate)
	if !ok {
		return HitRate{Value: nil, Display: emDash}
	}
	p := pct
	return HitRate{Value: &p, Display: strconv.FormatFloat(pct, 'f', 1, 64) + "%"}
}

func satAdd(a, b int64) int64 {
	if b > 0 && a > math.MaxInt64-b {
		return math.MaxInt64
	}
	if b < 0 && a < math.MinInt64-b {
		return math.MinInt64
	}
	return a + b
}

// FormatCompact renders n as a K/M/B/T figure. Negatives become "—".
func FormatCompact(n int64) string {
	if n < 0 {
		return emDash
	}
	if n < 1000 {
		return strconv.FormatInt(n, 10)
	}
	switch {
	case n >= 1_000_000_000_000:
		return formatScaled(n, 1_000_000_000_000, "T")
	case n >= 1_000_000_000:
		return formatScaled(n, 1_000_000_000, "B")
	case n >= 1_000_000:
		return formatScaled(n, 1_000_000, "M")
	default:
		return formatScaled(n, 1_000, "K")
	}
}

func formatScaled(n, div int64, suffix string) string {
	whole := n / div
	rem := n % div
	frac := (rem*100 + div/2) / div
	if frac >= 100 {
		whole++
		frac = 0
	}
	if whole >= 1000 && suffix != "T" {
		return FormatCompact(whole * div)
	}
	if frac == 0 {
		return strconv.FormatInt(whole, 10) + suffix
	}
	s := strconv.FormatInt(whole, 10) + "." + twoDigits(frac)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s + suffix
}

func twoDigits(n int64) string {
	if n < 10 {
		return "0" + strconv.FormatInt(n, 10)
	}
	return strconv.FormatInt(n, 10)
}
