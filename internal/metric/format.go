package metric

import (
	"fmt"
	"strconv"
)

func FormatM(tokens int64) string {
	m := float64(tokens) / 1_000_000
	if tokens != 0 && m < 0.01 {
		return fmt.Sprintf("%.4f M", m)
	}
	return fmt.Sprintf("%.2f M", m)
}

func FormatCount(n int64) string {
	s := strconv.FormatInt(n, 10)
	neg := false
	if s[0] == '-' {
		neg = true
		s = s[1:]
	}
	var b []byte
	for i, c := range []byte(s) {
		if i > 0 && (len(s)-i)%3 == 0 {
			b = append(b, ',')
		}
		b = append(b, c)
	}
	if neg {
		return "-" + string(b)
	}
	return string(b)
}

func HitRate(miss, cacheRead, cacheCreate int64) (float64, bool) {
	den := cacheRead + miss + cacheCreate
	if den == 0 {
		return 0, false
	}
	return 100 * float64(cacheRead) / float64(den), true
}
