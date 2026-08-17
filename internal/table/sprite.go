package table

import (
	"strings"
	"time"
)

const (
	lemon     = "\x1b[38;5;228m" // soft lemon, not Claude orange 208
	spriteW   = 7
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

// SpriteTick walks one pose every 180ms (same cadence as the dashboard kid).
func SpriteTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(180*time.Millisecond)) % poseCount
}

func SpritePose(tick int) int {
	return mod(tick, poseCount)
}

// SpriteMood is the fidget label that sits on the last sprite row.
func SpriteMood(tick int, ascii bool) string {
	if ascii {
		switch SpritePose(tick) {
		case PoseScratch:
			return "scratch"
		case PoseAbacus:
			return "abacus"
		case PoseToss:
			return "toss"
		case PoseFire:
			return "fire"
		case PoseBlink:
			return "blink"
		default:
			return "grin"
		}
	}
	switch SpritePose(tick) {
	case PoseScratch:
		return "挠头"
	case PoseAbacus:
		return "拨算盘"
	case PoseToss:
		return "投煤"
	case PoseFire:
		return "煅烧"
	case PoseBlink:
		return "眨眼"
	default:
		return "出窑"
	}
}

// SpriteLines is a 4-line kiln kid. Caption sits on the first line; mood on the last.
func SpriteLines(tick int, caption string, ascii bool) []string {
	body := spriteFrame(tick, ascii)
	out := make([]string, len(body))
	copy(out, body)
	if caption != "" {
		out[0] = out[0] + spriteGap + caption
	}
	if mood := SpriteMood(tick, ascii); mood != "" {
		last := len(out) - 1
		out[last] = out[last] + spriteGap + mood
	}
	return out
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	return composeSprite(spriteFrame(tick, ascii), extrasFor(tick, caption, "", ascii), color)
}

// SpriteHUD is the live scan scene: caption, charge bar, mood.
func SpriteHUD(tick int, caption string, index, total int, ascii, color bool) string {
	return composeSprite(spriteFrame(tick, ascii), extrasFor(tick, caption, ChargeBar(index, total, ascii), ascii), color)
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

func extrasFor(tick int, caption, bar string, ascii bool) []string {
	mood := SpriteMood(tick, ascii)
	return []string{caption, bar, "", mood}
}

func composeSprite(body []string, extras []string, color bool) string {
	var b strings.Builder
	for i, row := range body {
		extra := ""
		if i < len(extras) && extras[i] != "" {
			extra = spriteGap + extras[i]
		}
		paint := Lemon
		if color {
			if i == 0 {
				paint = Ember
			} else {
				paint = Dim
			}
			b.WriteString(Lemon(row, true))
			if extra != "" {
				b.WriteString(paint(extra, true))
			}
		} else {
			b.WriteString(row)
			b.WriteString(extra)
		}
		b.WriteByte('\n')
	}
	return b.String()
}

func spriteFrame(tick int, ascii bool) []string {
	pose := SpritePose(tick)
	if ascii {
		return asciiFrame(pose)
	}
	return unicodeFrame(pose)
}

func unicodeFrame(pose int) []string {
	// each row is spriteW cells; tuft ∩∩, pot feet ∪∪
	switch pose {
	case PoseScratch:
		return []string{
			"  ∩∩~  ",
			" (•ᴗ•) ",
			" /|~|\\ ",
			"  ∪∪   ",
		}
	case PoseAbacus:
		return []string{
			"  ∩∩   ",
			" (•ᴗ•) ",
			" /|≡|\\ ",
			"  ∪∪   ",
		}
	case PoseToss:
		return []string{
			"  ∩∩*  ",
			" (•ᴗ•) ",
			" /| *\\ ",
			"  ∪∪   ",
		}
	case PoseFire:
		return []string{
			"  ∩∩   ",
			" (✧ᴗ✧) ",
			" /|∩|\\ ",
			"  ∪∪   ",
		}
	case PoseBlink:
		return []string{
			"  ∩∩   ",
			" (•-•) ",
			" /|  \\ ",
			"  ∪∪   ",
		}
	default: // grin
		return []string{
			"  ∩∩   ",
			" (✧ᴗ✧) ",
			" /|  \\ ",
			"  ∪∪   ",
		}
	}
}

func asciiFrame(pose int) []string {
	switch pose {
	case PoseScratch:
		return []string{
			"  /~\\~ ",
			" (o_o) ",
			" /|~|\\ ",
			"  \\_/  ",
		}
	case PoseAbacus:
		return []string{
			"  /~\\  ",
			" (o_o) ",
			" /|#|\\ ",
			"  \\_/  ",
		}
	case PoseToss:
		return []string{
			"  /~\\* ",
			" (o_o) ",
			" /| *\\ ",
			"  \\_/  ",
		}
	case PoseFire:
		return []string{
			"  /~\\  ",
			" (^_^) ",
			" /|n|\\ ",
			"  \\_/  ",
		}
	case PoseBlink:
		return []string{
			"  /~\\  ",
			" (o_o) ",
			" /|  \\ ",
			"  \\_/  ",
		}
	default:
		return []string{
			"  /~\\  ",
			" (^_^) ",
			" /|  \\ ",
			"  \\_/  ",
		}
	}
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
