package table

import (
	"strings"
	"testing"
	"time"
)

func TestSpriteFaceIsThreeRowsSameWidth(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for tick := 0; tick < poseCount*2; tick++ {
			body := spriteFrame(tick, 0, ascii)
			if len(body) != 3 {
				t.Fatalf("ascii=%v tick=%d lines=%d", ascii, tick, len(body))
			}
			for i, line := range body {
				if DisplayWidth(line) != spriteW {
					t.Fatalf("ascii=%v tick=%d line %d width %d want %d %q", ascii, tick, i, DisplayWidth(line), spriteW, line)
				}
				if strings.ContainsAny(line, "▁▂▃▄▅▆") {
					t.Fatalf("face must not look like a spark: %q", line)
				}
			}
		}
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
	if strings.Contains(lines[0], "拨珠中") {
		t.Fatalf("mood leaked onto roof: %q", lines[0])
	}
}

func TestSpriteASCIIHasNoBlockFace(t *testing.T) {
	for tick := 0; tick < poseCount; tick++ {
		block := SpriteBlock(tick, "reading", true, false)
		if strings.ContainsAny(block, "•╭╰█≡") {
			t.Fatalf("ascii leaked: %q", block)
		}
		if !strings.Contains(block, ".--.") && !strings.Contains(block, ".^^.") && !strings.Contains(block, ".-*.") {
			t.Fatalf("ascii lost the face: %q", block)
		}
	}
}

func TestSpriteColorIsLemonNotClaudeOrange(t *testing.T) {
	got := SpriteBlock(PoseGrin, "hi", false, true)
	if !strings.Contains(got, "38;5;227") {
		t.Fatalf("want lemon 227: %q", got)
	}
	if strings.Contains(got, "38;5;208") || strings.Contains(got, "38;5;228") {
		t.Fatal("must not use Claude orange 208 or pale 228")
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
	if SpriteMood(PoseToss, false) != "搬煤中" {
		t.Fatal(SpriteMood(PoseToss, false))
	}
	if SpriteMood(PoseScratch, false) != "挠头中" {
		t.Fatal(SpriteMood(PoseScratch, false))
	}
}

func TestChargeBarFills(t *testing.T) {
	got := ChargeBar(3, 6, false)
	if DisplayWidth(got) != 8 || strings.Count(got, "▰") != 4 {
		t.Fatal(got)
	}
	if ChargeBar(2, 8, true) != "[==------]" {
		t.Fatal(ChargeBar(2, 8, true))
	}
}

func TestSpriteHUDIsThreeLines(t *testing.T) {
	block := SpriteHUD(PoseAbacus, PoseAbacus, "正在读 Codex…  2/6", 2, 6, false, false)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(lines) != 3 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") || !strings.Contains(lines[1], "拨珠中") || !strings.Contains(lines[2], "▰") {
		t.Fatalf("%#v", lines)
	}
}
