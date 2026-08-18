package table

import (
	"strings"
	"testing"
	"time"
)

func TestClawdFaceIsThreeRowsSameWidth(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for tick := 0; tick < poseCount; tick++ {
			body := clawdFace(tick, ascii)
			if len(body) != 3 {
				t.Fatalf("ascii=%v tick=%d lines=%d", ascii, tick, len(body))
			}
			for i, line := range body {
				if DisplayWidth(line) != spriteW {
					t.Fatalf("ascii=%v tick=%d line %d width %d want %d %q", ascii, tick, i, DisplayWidth(line), spriteW, line)
				}
			}
		}
	}
}

func TestClawdHasTwoEyeSlots(t *testing.T) {
	body := clawdFace(PoseGrin, false)
	if !strings.Contains(body[1], "▌") {
		t.Fatalf("eyes: %#v", body)
	}
	if strings.Count(body[1], "▌") != 2 {
		t.Fatalf("want two eye bars: %q", body[1])
	}
}

func TestSpriteCaptionSitsOnFirstLineMoodOnSecond(t *testing.T) {
	lines := SpriteLines(PoseAbacus, "正在读 Codex… 2/6", false)
	if len(lines) != 3 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
	if !strings.Contains(lines[1], "拨珠中") {
		t.Fatalf("mood: %#v", lines)
	}
}

func TestSpriteASCIIHasNoBlocks(t *testing.T) {
	block := SpriteBlock(0, "reading", true, false)
	if strings.ContainsAny(block, "▄█▀▌") {
		t.Fatalf("ascii leaked: %q", block)
	}
	if !strings.Contains(block, "+------+") {
		t.Fatalf("ascii lost the slab: %q", block)
	}
}

func TestSpriteColorIsGoldNotClaudeOrange(t *testing.T) {
	got := SpriteBlock(PoseGrin, "hi", false, true)
	if !strings.Contains(got, "38;2;255;215;0") {
		t.Fatalf("want #FFD700: %q", got)
	}
	if strings.Contains(got, "38;5;208") {
		t.Fatal("must not use Claude orange 208")
	}
}

func TestSpriteTickWalks(t *testing.T) {
	if SpriteTick(0) == SpriteTick(400*time.Millisecond) {
		t.Fatal("tick should advance")
	}
}

func TestLemonNoColor(t *testing.T) {
	if Lemon("x", false) != "x" {
		t.Fatal(Lemon("x", false))
	}
}

func TestSpriteMoodGerund(t *testing.T) {
	if SpriteMood(PoseScratch, false) != "挠头中" {
		t.Fatal(SpriteMood(PoseScratch, false))
	}
}

func TestChargeBarFills(t *testing.T) {
	if ChargeBar(2, 8, true) != "[==------]" {
		t.Fatal(ChargeBar(2, 8, true))
	}
}

func TestSpriteHUDKeepsGerund(t *testing.T) {
	block := SpriteHUD(PoseGrin, PoseScratch, "正在读 Codex…  2/6", 2, 6, false, false)
	if !strings.Contains(block, "挠头中") || !strings.Contains(block, "正在读 Codex") {
		t.Fatalf("%q", block)
	}
	if !strings.Contains(block, "▄██████▄") {
		t.Fatalf("missing clawd slab:\n%s", block)
	}
}
