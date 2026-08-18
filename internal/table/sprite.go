package table

import (
	"strings"
	"time"
)

const (
	lemon     = "\x1b[38;5;227m" // lemon, not Claude orange 208
	spriteW   = 7                // Kimi welcome logo width
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

// Copied from MoonshotAI/kimi-code welcome.ts — the 2-line block mark at the top.
var kimiLogo = []string{"▐█▛█▛█▌", "▐█████▌"}
var kimiLogoASCII = []string{"|#|#|#|", "|#####|"}

// Copied from MoonshotAI/kimi-code rendering.ts MOON_SPINNER_FRAMES (120ms).
var moonGlyphs = []string{"🌑", "🌒", "🌓", "🌔", "🌕", "🌖", "🌗", "🌘"}
var moonGlyphsASCII = []string{"( )", "(@)", "(:)", "(o)", "(O)", "(o)", "(:)", "(@)"}

func SpriteTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(120*time.Millisecond)) % len(moonGlyphs)
}

func SpriteFlap(elapsed time.Duration) int {
	return SpriteTick(elapsed) % 2
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

func kimiMark(ascii bool) []string {
	if ascii {
		return append([]string(nil), kimiLogoASCII...)
	}
	return append([]string(nil), kimiLogo...)
}

func MoonGlyph(tick int, ascii bool) string {
	if ascii {
		return moonGlyphsASCII[mod(tick, len(moonGlyphsASCII))]
	}
	return moonGlyphs[mod(tick, len(moonGlyphs))]
}

func SpriteLines(tick int, caption string, ascii bool) []string {
	return attach(kimiMark(ascii), caption, SpriteMood(tick, ascii))
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	return paintFace(kimiMark(ascii), caption, "", color)
}

func SpriteScene(tick int, line0, line1, line2 string, ascii, color bool) string {
	_ = tick
	_ = line2
	return paintFace(kimiMark(ascii), line0, line1, color)
}

// SpriteHUD is Kimi's moon-loader: one spinning moon + label.
func SpriteHUD(tick, moodTick int, caption string, index, total int, ascii, color bool) string {
	moon := MoonGlyph(tick, ascii)
	mood := SpriteMood(moodTick, ascii)
	bar := ChargeBar(index, total, ascii)
	parts := []string{moon, mood}
	if caption != "" {
		parts = append(parts, caption)
	}
	if bar != "" {
		parts = append(parts, bar)
	}
	line := strings.Join(parts, spriteGap)
	if !color {
		return line + "\n"
	}
	var b strings.Builder
	b.WriteString(Lemon(moon, true))
	b.WriteString(spriteGap)
	b.WriteString(Ember(mood, true))
	if caption != "" {
		b.WriteString(spriteGap)
		b.WriteString(Dim(caption, true))
	}
	if bar != "" {
		b.WriteString(spriteGap)
		b.WriteString(Dim(bar, true))
	}
	b.WriteByte('\n')
	return b.String()
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

func attach(body []string, a, b string) []string {
	out := append([]string(nil), body...)
	if len(out) == 0 {
		return out
	}
	if a != "" {
		out[0] += spriteGap + a
	}
	if b != "" && len(out) > 1 {
		out[1] += spriteGap + b
	}
	return out
}

func paintFace(body []string, a, b string, color bool) string {
	lines := attach(body, a, b)
	if !color {
		return strings.Join(lines, "\n") + "\n"
	}
	var bld strings.Builder
	for i, line := range lines {
		face, extra, ok := strings.Cut(line, spriteGap)
		if !ok {
			bld.WriteString(Lemon(line, true))
			bld.WriteByte('\n')
			continue
		}
		bld.WriteString(Lemon(face, true))
		bld.WriteString(spriteGap)
		if i == 0 {
			bld.WriteString(Ember(extra, true))
		} else {
			bld.WriteString(Dim(extra, true))
		}
		bld.WriteByte('\n')
	}
	return bld.String()
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
