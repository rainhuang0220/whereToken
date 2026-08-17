package table

import (
	"strings"
	"testing"
	"time"
)

func TestSpriteLinesAreTwoRowsSameWidth(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for tick := 0; tick < 12; tick++ {
			lines := SpriteLines(tick, "", ascii)
			if len(lines) != 2 {
				t.Fatalf("ascii=%v tick=%d lines=%d", ascii, tick, len(lines))
			}
			if DisplayWidth(lines[0]) != DisplayWidth(lines[1]) {
				t.Fatalf("ascii=%v tick=%d widths %d vs %d\n%q\n%q", ascii, tick, DisplayWidth(lines[0]), DisplayWidth(lines[1]), lines[0], lines[1])
			}
		}
	}
}

func TestSpriteCaptionSitsOnFirstLine(t *testing.T) {
	lines := SpriteLines(0, "正在读 Codex… 2/6", false)
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
	if strings.Contains(lines[1], "正在读") {
		t.Fatalf("caption leaked onto legs: %q", lines[1])
	}
}

func TestSpriteASCIIHasNoWideToys(t *testing.T) {
	for tick := 0; tick < 8; tick++ {
		block := SpriteBlock(tick, "reading", true, false)
		if strings.ContainsAny(block, "•ᴗω✧≡") {
			t.Fatalf("ascii leaked unicode: %q", block)
		}
	}
}

func TestSpriteColorIsLemonNotClaudeOrange(t *testing.T) {
	got := SpriteBlock(0, "hi", false, true)
	if !strings.Contains(got, "38;5;228") {
		t.Fatalf("want lemon 228: %q", got)
	}
	if strings.Contains(got, "38;5;208") {
		t.Fatal("must not use Claude kiln orange 208")
	}
}

func TestSpriteTickWalks(t *testing.T) {
	a := SpriteTick(0)
	b := SpriteTick(400 * time.Millisecond)
	if a == b {
		t.Fatalf("tick should advance: %d %d", a, b)
	}
}

func TestLemonNoColor(t *testing.T) {
	if Lemon("x", false) != "x" {
		t.Fatal(Lemon("x", false))
	}
}
