package table

import "unicode"

func DisplayWidth(s string) int {
	n := 0
	for _, r := range s {
		n += runeWidth(r)
	}
	return n
}

func PadRight(s string, width int) string {
	pad := width - DisplayWidth(s)
	if pad <= 0 {
		return s
	}
	b := make([]byte, 0, len(s)+pad)
	b = append(b, s...)
	for i := 0; i < pad; i++ {
		b = append(b, ' ')
	}
	return string(b)
}

func Truncate(s string, width int) string {
	return TruncateEllipsis(s, width, "…")
}

func TruncateEllipsis(s string, width int, ell string) string {
	if width <= 0 {
		return ""
	}
	if DisplayWidth(s) <= width {
		return s
	}
	if ell == "" {
		ell = "…"
	}
	ew := DisplayWidth(ell)
	if width <= ew {
		return truncateRunes(ell, width)
	}
	limit := width - ew
	n := 0
	var b []rune
	for _, r := range s {
		w := runeWidth(r)
		if n+w > limit {
			break
		}
		b = append(b, r)
		n += w
	}
	return string(b) + ell
}

func truncateRunes(s string, width int) string {
	n := 0
	var b []rune
	for _, r := range s {
		w := runeWidth(r)
		if n+w > width {
			break
		}
		b = append(b, r)
		n += w
	}
	return string(b)
}

func PadLeft(s string, width int) string {
	pad := width - DisplayWidth(s)
	if pad <= 0 {
		return s
	}
	b := make([]byte, 0, len(s)+pad)
	for i := 0; i < pad; i++ {
		b = append(b, ' ')
	}
	b = append(b, s...)
	return string(b)
}

func runeWidth(r rune) int {
	if r == 0 || r == '\u200b' {
		return 0
	}
	if unicode.Is(unicode.Mn, r) || unicode.Is(unicode.Me, r) || unicode.Is(unicode.Cc, r) {
		return 0
	}
	if isWide(r) {
		return 2
	}
	return 1
}

func isWide(r rune) bool {
	switch {
	case r >= 0x1100 && r <= 0x115F:
		return true
	case r == 0x2329 || r == 0x232A:
		return true
	case r >= 0x2E80 && r <= 0xA4CF && r != 0x303F:
		return true
	case r >= 0xAC00 && r <= 0xD7A3:
		return true
	case r >= 0xF900 && r <= 0xFAFF:
		return true
	case r >= 0xFE10 && r <= 0xFE19:
		return true
	case r >= 0xFE30 && r <= 0xFE6F:
		return true
	case r >= 0xFF00 && r <= 0xFF60:
		return true
	case r >= 0xFFE0 && r <= 0xFFE6:
		return true
	case r >= 0x1F300 && r <= 0x1F64F:
		return true
	case r >= 0x1F680 && r <= 0x1F6FF:
		return true
	case r >= 0x1F900 && r <= 0x1F9FF:
		return true
	case r >= 0x1FA00 && r <= 0x1FAFF:
		return true
	case r >= 0x20000 && r <= 0x3FFFD:
		return true
	default:
		return false
	}
}
