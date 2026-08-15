package metric

import (
	"fmt"
	"strconv"
	"strings"
)

func FormatM(tokens int64) string {
	m := float64(tokens) / 1_000_000
	var s string
	if tokens != 0 && m > -0.01 && m < 0.01 {
		s = fmt.Sprintf("%.4f", m)
	} else {
		s = fmt.Sprintf("%.2f", m)
	}
	return groupFloat(s) + " M"
}

func groupFloat(s string) string {
	neg := false
	if strings.HasPrefix(s, "-") {
		neg = true
		s = s[1:]
	}
	intPart, frac, ok := strings.Cut(s, ".")
	n, err := strconv.ParseInt(intPart, 10, 64)
	if err != nil {
		if neg {
			return "-" + s
		}
		return s
	}
	g := FormatCount(n)
	if neg {
		g = "-" + g
	}
	if !ok {
		return g
	}
	return g + "." + frac
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

func FormatShare(part, total int64) string {
	if total <= 0 {
		return "—"
	}
	return fmt.Sprintf("%.1f%%", 100*float64(part)/float64(total))
}

func HitRate(miss, cacheRead, cacheCreate int64) (float64, bool) {
	den := cacheRead + miss + cacheCreate
	if den == 0 {
		return 0, false
	}
	return 100 * float64(cacheRead) / float64(den), true
}
