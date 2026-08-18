package table

import (
	"strings"
	"time"
)

const (
	lemon     = "\x1b[38;5;228m" // soft lemon, not Claude orange 208
	spriteW   = 2                // one CJK cell: Kimi moon / Claude asterisk size
	spriteGap = "  "
)

func Lemon(s string, color bool) string {
	if !color || s == "" {
		return s
	}
	return lemon + s + "\x1b[0m"
}

const (
	PoseScratch = iota
	PoseAbacus
	PoseToss
	PoseFire
	PoseBlink
	PoseGrin
	poseCount
)

// SpriteTick walks the 2-cell kiln mark (Kimi moon cadence: 120ms × 8).
func SpriteTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(120*time.Millisecond)) % len(unicodeGlyphs)
}

// SpriteFlap kept so older HUD call sites compile; the mark itself is the motion.
func SpriteFlap(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(120*time.Millisecond)) % 2
}

func SpritePose(tick int) int {
	return mod(tick, poseCount)
}

func SpriteMoodTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(400*time.Millisecond)) % poseCount
}

// SpriteMood is the gerund next to the mark: 挠头中 / 搬煤中.
func SpriteMood(tick int, ascii bool) string {
	if ascii {
		switch SpritePose(tick) {
		case PoseScratch:
			return "scratching"
		case PoseAbacus:
			return "counting"
		case PoseToss:
			return "hauling"
		case PoseFire:
			return "firing"
		case PoseBlink:
			return "blinking"
		default:
			return "done"
		}
	}
	switch SpritePose(tick) {
	case PoseScratch:
		return "挠头中"
	case PoseAbacus:
		return "拨珠中"
	case PoseToss:
		return "搬煤中"
	case PoseFire:
		return "煅烧中"
	case PoseBlink:
		return "眨眼中"
	default:
		return "出窑中"
	}
}

var unicodeGlyphs = []string{"▛▜", "▜█", "█▟", "▟█", "▙▟", "█▙", "▛█", "█▜"}
var asciiGlyphs = []string{"##", "[]", "<>", "%%", "**", "++", "==", "oo"}

// KilnGlyph is a 2-cell block mark. Empty quadrants are the eyes; they rotate like a moon.
func KilnGlyph(tick int, ascii bool) string {
	if ascii {
		return asciiGlyphs[mod(tick, len(asciiGlyphs))]
	}
	return unicodeGlyphs[mod(tick, len(unicodeGlyphs))]
}

// SpriteLines is one status line: mark + mood + caption.
func SpriteLines(tick int, caption string, ascii bool) []string {
	return []string{plainMark(KilnGlyph(tick, ascii), SpriteMood(tick, ascii), caption, "")}
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	return markLine(KilnGlyph(tick, ascii), "", caption, "", color) + "\n"
}

// SpriteHUD is one Kimi/Claude-style activity line.
func SpriteHUD(tick, moodTick int, caption string, index, total int, ascii, color bool) string {
	return markLine(KilnGlyph(tick, ascii), SpriteMood(moodTick, ascii), caption, ChargeBar(index, total, ascii), color) + "\n"
}

func ChargeBar(index, total int, ascii bool) string {
	const w = 8
	if total <= 0 {
		return ""
	}
	n := index * w / total
	if n < 0 {
		n = 0
	}
	if n > w {
		n = w
	}
	if ascii {
		return "[" + strings.Repeat("=", n) + strings.Repeat("-", w-n) + "]"
	}
	return strings.Repeat("▰", n) + strings.Repeat("▱", w-n)
}

func plainMark(glyph, mood, caption, bar string) string {
	parts := []string{glyph}
	if mood != "" {
		parts = append(parts, mood)
	}
	if caption != "" {
		parts = append(parts, caption)
	}
	if bar != "" {
		parts = append(parts, bar)
	}
	return strings.Join(parts, spriteGap)
}

func markLine(glyph, mood, caption, bar string, color bool) string {
	if !color {
		return plainMark(glyph, mood, caption, bar)
	}
	var b strings.Builder
	b.WriteString(Lemon(glyph, true))
	writePart := func(s string, paint func(string, bool) string) {
		if s == "" {
			return
		}
		b.WriteString(spriteGap)
		b.WriteString(paint(s, true))
	}
	writePart(mood, Ember)
	writePart(caption, Dim)
	writePart(bar, Dim)
	return b.String()
}

func mod(n, m int) int {
	if m <= 0 {
		return 0
	}
	n = n % m
	if n < 0 {
		n += m
	}
	return n
}
