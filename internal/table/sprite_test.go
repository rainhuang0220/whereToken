package table

import (
	"strings"
	"testing"
	"time"
)

func TestKilnGlyphIsOneCJKCell(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		for tick := 0; tick < 16; tick++ {
			g := KilnGlyph(tick, ascii)
			if DisplayWidth(g) != spriteW {
				t.Fatalf("ascii=%v tick=%d width %d %q", ascii, tick, DisplayWidth(g), g)
			}
		}
	}
}

func TestSpriteLinesAreOneStatusRow(t *testing.T) {
	lines := SpriteLines(PoseAbacus, "正在读 Codex… 2/6", false)
	if len(lines) != 1 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
	if !strings.Contains(lines[0], "拨珠中") {
		t.Fatalf("mood missing: %q", lines[0])
	}
	if strings.Contains(lines[0], "\n") {
		t.Fatal("status line must be one row")
	}
}

func TestSpriteASCIIHasNoBlockMark(t *testing.T) {
	for tick := 0; tick < 8; tick++ {
		block := SpriteBlock(tick, "reading", true, false)
		if strings.ContainsAny(block, "•ᴗ✧≡∩∪▛▜▙▟█") {
			t.Fatalf("ascii leaked: %q", block)
		}
		if !strings.ContainsAny(block, "#=[]<>%*+o") {
			t.Fatalf("ascii lost the mark: %q", block)
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
	hud := SpriteHUD(0, PoseScratch, "hi", 1, 2, false, true)
	if !strings.Contains(hud, "1;38;5;228") {
		t.Fatalf("mood should be bold lemon: %q", hud)
	}
	if !strings.Contains(hud, "\x1b[2m") {
		t.Fatalf("caption should dim: %q", hud)
	}
}

func TestSpriteTickWalks(t *testing.T) {
	a := SpriteTick(0)
	b := SpriteTick(400 * time.Millisecond)
	if a == b {
		t.Fatalf("tick should advance: %d %d", a, b)
	}
	if SpriteTick(0) != 0 {
		t.Fatalf("tick 0: %d", SpriteTick(0))
	}
}

func TestLemonNoColor(t *testing.T) {
	if Lemon("x", false) != "x" {
		t.Fatal(Lemon("x", false))
	}
}

func TestSpriteMoodGerund(t *testing.T) {
	if SpriteMood(PoseAbacus, true) != "counting" {
		t.Fatal(SpriteMood(PoseAbacus, true))
	}
	if SpriteMood(PoseToss, false) != "搬煤中" {
		t.Fatal(SpriteMood(PoseToss, false))
	}
	if SpriteMood(PoseScratch, false) != "挠头中" {
		t.Fatal(SpriteMood(PoseScratch, false))
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

func TestSpriteHUDIsOneLine(t *testing.T) {
	block := SpriteHUD(0, PoseAbacus, "正在读 Codex…  2/6", 2, 6, false, false)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("caption: %q", lines[0])
	}
	if !strings.Contains(lines[0], "▰") {
		t.Fatalf("bar: %q", lines[0])
	}
	if !strings.Contains(lines[0], "拨珠中") {
		t.Fatalf("mood: %q", lines[0])
	}
}
