package metric

import "fmt"

func FormatM(tokens int64) string {
	m := float64(tokens) / 1_000_000
	if tokens != 0 && m < 0.01 {
		return fmt.Sprintf("%.4f M", m)
	}
	return fmt.Sprintf("%.2f M", m)
}

func HitRate(miss, cacheRead, cacheCreate int64) (float64, bool) {
	den := cacheRead + miss + cacheCreate
	if den == 0 {
		return 0, false
	}
	return 100 * float64(cacheRead) / float64(den), true
}
