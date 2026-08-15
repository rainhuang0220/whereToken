package table

import "strings"

const (
	sparkUnicode = "▁▂▃▄▅▆▇█"
	sparkASCII   = "._-=+*#"
)

func Spark(values []int64, ascii bool) string {
	glyphs := []rune(sparkUnicode)
	if ascii {
		glyphs = []rune(sparkASCII)
	}
	if len(values) == 0 {
		return ""
	}
	var max int64
	for _, v := range values {
		if v > max {
			max = v
		}
	}
	var b strings.Builder
	n := len(glyphs)
	for _, v := range values {
		if v < 0 {
			v = 0
		}
		idx := 0
		if max > 0 {
			idx = int(float64(v) / float64(max) * float64(n-1))
			if idx >= n {
				idx = n - 1
			}
		}
		b.WriteRune(glyphs[idx])
	}
	return b.String()
}
