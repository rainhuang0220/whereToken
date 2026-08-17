package table

import (
	"strings"
	"time"
)

const lemon = "\x1b[38;5;228m" // soft lemon, not Claude orange 208

func Lemon(s string, color bool) string {
	if !color || s == "" {
		return s
	}
	return lemon + s + "\x1b[0m"
}

// SpriteLines is a 2-line kiln sprite plus a progress caption on the first line.
func SpriteLines(tick int, caption string, ascii bool) []string {
	body := spriteFrame(tick, ascii)
	if caption == "" {
		return body
	}
	out := make([]string, len(body))
	copy(out, body)
	out[0] = out[0] + "  " + caption
	return out
}

func SpriteBlock(tick int, caption string, ascii, color bool) string {
	lines := SpriteLines(tick, caption, ascii)
	if color {
		for i := range lines {
			lines[i] = Lemon(lines[i], true)
		}
	}
	return strings.Join(lines, "\n") + "\n"
}

func SpriteTick(elapsed time.Duration) int {
	if elapsed < 0 {
		elapsed = 0
	}
	return int(elapsed/(180*time.Millisecond)) % 12
}

func spriteFrame(tick int, ascii bool) []string {
	if ascii {
		frames := [][]string{
			{" (o_o) ", " /|~|\\ "},
			{" (o_o) ", " /|#|\\ "},
			{" (oO)  ", " /|*|\\ "},
			{" (o_o) ", " /|=|\\ "},
			{" (o_o) ", " /|~|\\ "},
			{" (^_^) ", " /| |\\ "},
		}
		return frames[mod(tick, len(frames))]
	}
	// kiln kid: scratch, abacus, toss-in-kiln, grin
	frames := [][]string{
		{" (•ᴗ•) ", " /|~|\\ "}, // scratch
		{" (•ᴗ•) ", " /|≡|\\ "}, // abacus
		{" (•ᴗ•) ", " /|*|\\ "}, // coal in
		{" (•ᴗ•) ", " /|∩|\\ "}, // kiln mouth
		{" (•-•) ", " /|~|\\ "},
		{" (✧ᴗ✧) ", " /| |\\ "}, // grin
	}
	return frames[mod(tick, len(frames))]
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
