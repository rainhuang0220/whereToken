package table

import (
	"strings"
	"time"
)

const (
	lemon     = "\x1b[38;5;227m" // deeper lemon than pale 228; not Claude orange 208
	spriteW   = 4
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

// SpriteLines is a 3-line kiln face. Caption sits on the first line; mood on the second.
func SpriteLines(tick int, caption string, ascii bool) []string {
	return attach(spriteFrame(tick, 0, ascii), caption, SpriteMood(tick, ascii), "")
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	return paintFace(spriteFrame(tick, 0, ascii), caption, "", "", color)
}

func SpriteScene(tick int, line0, line1, line2 string, ascii, color bool) string {
	return paintFace(spriteFrame(tick, 0, ascii), line0, line1, line2, color)
}

func SpriteHUD(tick, moodTick int, caption string, index, total int, ascii, color bool) string {
	return paintFace(spriteFrame(tick, 0, ascii), caption, SpriteMood(moodTick, ascii), ChargeBar(index, total, ascii), color)
}

func spriteFrame(tick int, flap int, ascii bool) []string {
	pose := SpritePose(tick)
	if ascii {
		return asciiFace(pose, flap)
	}
	return unicodeFace(pose, flap)
}

func unicodeFace(pose, flap int) []string {
	// 4-cell kiln brick with hole eyes — a face, not a spark bar.
	switch pose {
	case PoseScratch:
		return []string{"╭──╮", "│•~│", "╰██╯"}
	case PoseAbacus:
		return []string{"╭──╮", "│≡≡│", "╰██╯"}
	case PoseToss:
		return []string{"╭─*╮", "│••│", "╰██╯"}
	case PoseFire:
		return []string{"╭^^╮", "│••│", "╰██╯"}
	case PoseBlink:
		return []string{"╭──╮", "│──│", "╰██╯"}
	default:
		if flap%2 == 1 {
			return []string{"╭──╮", "│••│", "╰██╯"}
		}
		return []string{"╭──╮", "│··│", "╰██╯"}
	}
}

func asciiFace(pose, flap int) []string {
	switch pose {
	case PoseScratch:
		return []string{".--.", "|o~|", "|__|"}
	case PoseAbacus:
		return []string{".--.", "|##|", "|__|"}
	case PoseToss:
		return []string{".-*.", "|oo|", "|__|"}
	case PoseFire:
		return []string{".^^.", "|oo|", "|__|"}
	case PoseBlink:
		return []string{".--.", "|--|", "|__|"}
	default:
		if flap%2 == 1 {
			return []string{".--.", "|oo|", "|__|"}
		}
		return []string{".--.", "|..|", "|__|"}
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
	if len(out) < 3 {
		return out
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
		switch i {
		case 0:
			bld.WriteString(Ember(extra, true))
		default:
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
