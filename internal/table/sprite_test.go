package table

import (
	"strings"
	"testing"
	"time"
)

func TestSpriteLinesAreFourRowsSameBodyWidth(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for tick := 0; tick < poseCount*2; tick++ {
			body := spriteFrame(tick, ascii)
			if len(body) != 4 {
				t.Fatalf("ascii=%v tick=%d lines=%d", ascii, tick, len(body))
			}
			w0 := DisplayWidth(body[0])
			if w0 != spriteW {
				t.Fatalf("ascii=%v tick=%d width %d want %d %q", ascii, tick, w0, spriteW, body[0])
			}
			for i, line := range body {
				if DisplayWidth(line) != w0 {
					t.Fatalf("ascii=%v tick=%d line %d width %d vs %d\n%q", ascii, tick, i, DisplayWidth(line), w0, line)
				}
			}
		}
	}
}

func TestSpriteCaptionSitsOnFirstLineMoodOnLast(t *testing.T) {
	lines := SpriteLines(PoseAbacus, "正在读 Codex… 2/6", false)
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
	if strings.Contains(lines[1], "正在读") || strings.Contains(lines[2], "正在读") {
		t.Fatalf("caption leaked: %#v", lines)
	}
	if !strings.Contains(lines[3], "拨算盘") {
		t.Fatalf("mood missing: %#v", lines)
	}
	if strings.Contains(lines[0], "拨算盘") {
		t.Fatalf("mood leaked onto tuft: %q", lines[0])
	}
}

func TestSpriteASCIIHasNoWideToys(t *testing.T) {
	for tick := 0; tick < poseCount; tick++ {
		block := SpriteBlock(tick, "reading", true, false)
		if strings.ContainsAny(block, "•ᴗω✧≡∩∪") {
			t.Fatalf("ascii leaked unicode: %q", block)
		}
		if !strings.Contains(block, "(o_o)") && !strings.Contains(block, "(^_^)") {
			t.Fatalf("ascii lost the face: %q", block)
		}
	}
}

func TestSpriteColorIsLemonNotClaudeOrange(t *testing.T) {
	got := SpriteBlock(PoseScratch, "hi", false, true)
	if !strings.Contains(got, "38;5;228") {
		t.Fatalf("want lemon 228: %q", got)
	}
	if strings.Contains(got, "38;5;208") {
		t.Fatal("must not use Claude kiln orange 208")
	}
	// caption is ember (bold lemon), mood is dim — not one flat wash
	if !strings.Contains(got, "1;38;5;228") {
		t.Fatalf("caption should be bold lemon: %q", got)
	}
	if !strings.Contains(got, "\x1b[2m") {
		t.Fatalf("mood should dim: %q", got)
	}
}

func TestSpriteTickWalks(t *testing.T) {
	a := SpriteTick(0)
	b := SpriteTick(400 * time.Millisecond)
	if a == b {
		t.Fatalf("tick should advance: %d %d", a, b)
	}
	if SpriteTick(0) != PoseScratch {
		t.Fatalf("tick 0 should scratch, got %d", SpriteTick(0))
	}
}

func TestLemonNoColor(t *testing.T) {
	if Lemon("x", false) != "x" {
		t.Fatal(Lemon("x", false))
	}
}

func TestSpriteMoodASCII(t *testing.T) {
	if SpriteMood(PoseAbacus, true) != "abacus" {
		t.Fatal(SpriteMood(PoseAbacus, true))
	}
	if SpriteMood(PoseToss, false) != "投煤" {
		t.Fatal(SpriteMood(PoseToss, false))
	}
}

func TestSpritePoseWraps(t *testing.T) {
	if SpritePose(poseCount) != PoseScratch {
		t.Fatal(SpritePose(poseCount))
	}
	if SpritePose(-1) != poseCount-1 {
		t.Fatal(SpritePose(-1))
	}
}

func TestChargeBarFills(t *testing.T) {
	if ChargeBar(0, 0, false) != "" {
		t.Fatal("empty total")
	}
	got := ChargeBar(3, 6, false)
	if DisplayWidth(got) != 8 {
		t.Fatalf("width %d %q", DisplayWidth(got), got)
	}
	if strings.Count(got, "▰") != 4 || strings.Count(got, "▱") != 4 {
		t.Fatalf("%q", got)
	}
	ascii := ChargeBar(2, 8, true)
	if ascii != "[==------]" {
		t.Fatal(ascii)
	}
}

func TestSpriteHUDPutsBarOnSecondLine(t *testing.T) {
	block := SpriteHUD(PoseAbacus, "正在读 Codex…  2/6", 2, 6, false, false)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(lines) != 4 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("caption: %q", lines[0])
	}
	if !strings.Contains(lines[1], "▰") {
		t.Fatalf("bar: %q", lines[1])
	}
	if !strings.Contains(lines[3], "拨算盘") {
		t.Fatalf("mood: %q", lines[3])
	}
}
