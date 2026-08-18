package table

import (
	"strings"
	"time"
)

const (
	// Gold lemon #FFD700. Not pale 228, not Claude orange 208.
	lemon     = "\x1b[38;2;255;215;0m"
	spriteW   = 8
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

func SpriteTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(160*time.Millisecond)) % poseCount
}

func SpriteFlap(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(160*time.Millisecond)) % 2
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

func SpriteLines(tick int, caption string, ascii bool) []string {
	return attach(clawdFace(tick, ascii), caption, SpriteMood(tick, ascii), "")
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	return paintFace(clawdFace(tick, ascii), caption, "", "", color)
}

func SpriteScene(tick int, line0, line1, line2 string, ascii, color bool) string {
	return paintFace(clawdFace(tick, ascii), line0, line1, line2, color)
}

func SpriteHUD(tick, moodTick int, caption string, index, total int, ascii, color bool) string {
	return paintFace(clawdFace(tick, ascii), caption, SpriteMood(moodTick, ascii), ChargeBar(index, total, ascii), color)
}

// clawdFace is Claude Code's block mascot: a slab with two vertical eye slots.
func clawdFace(tick int, ascii bool) []string {
	pose := SpritePose(tick)
	if ascii {
		switch pose {
		case PoseBlink:
			return []string{"+------+", "|      |", "+------+"}
		case PoseScratch:
			return []string{"+------+", "| |  ~ |", "+------+"}
		case PoseToss:
			return []string{"+------+", "| |  |*|", "+------+"}
		default:
			return []string{"+------+", "| |  | |", "+------+"}
		}
	}
	switch pose {
	case PoseBlink:
		return []string{"▄██████▄", "█ ▂  ▂ █", "▀██████▀"}
	case PoseScratch:
		return []string{"▄██████▄", "█ ▌  ~ █", "▀██████▀"}
	case PoseToss:
		return []string{"▄██████▄", "█ ▌  ▌*█", "▀██████▀"}
	case PoseFire:
		return []string{"▄██^^██▄", "█ ▌  ▌ █", "▀██████▀"}
	default:
		return []string{"▄██████▄", "█ ▌  ▌ █", "▀██████▀"}
	}
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

func attach(body []string, a, b, c string) []string {
	out := append([]string(nil), body...)
	for len(out) < 3 {
		out = append(out, strings.Repeat(" ", spriteW))
	}
	if a != "" {
		out[0] += spriteGap + a
	}
	if b != "" {
		out[1] += spriteGap + b
	}
	if c != "" {
		out[2] += spriteGap + c
	}
	return out
}

func paintFace(body []string, a, b, c string, color bool) string {
	lines := attach(body, a, b, c)
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
