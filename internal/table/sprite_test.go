package table

import (
	"strings"
	"testing"
	"time"
)

func TestKimiLogoIsTwoRowsSameWidth(t *testing.T) {
	for _, ascii := range []bool{false, true} {
		body := kimiMark(ascii)
		if len(body) != 2 {
			t.Fatalf("ascii=%v lines=%d", ascii, len(body))
		}
		if DisplayWidth(body[0]) != spriteW || DisplayWidth(body[1]) != spriteW {
			t.Fatalf("ascii=%v widths %d %d want %d\n%q\n%q", ascii, DisplayWidth(body[0]), DisplayWidth(body[1]), spriteW, body[0], body[1])
		}
	}
	if kimiLogo[0] != "▐█▛█▛█▌" || kimiLogo[1] != "▐█████▌" {
		t.Fatalf("must copy Kimi welcome logo, got %#v", kimiLogo)
	}
}

func TestSpriteCaptionSitsOnFirstLineMoodOnSecond(t *testing.T) {
	lines := SpriteLines(PoseAbacus, "正在读 Codex… 2/6", false)
	if len(lines) != 2 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
	if !strings.Contains(lines[1], "拨珠中") {
		t.Fatalf("mood: %#v", lines)
	}
}

func TestSpriteASCIIHasNoKimiBlocks(t *testing.T) {
	block := SpriteBlock(0, "reading", true, false)
	if strings.ContainsAny(block, "▐█▛▌🌑") {
		t.Fatalf("ascii leaked: %q", block)
	}
	if !strings.Contains(block, "|#|#|#|") {
		t.Fatalf("ascii lost the mark: %q", block)
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

func TestSpriteTickWalksMoon(t *testing.T) {
	if SpriteTick(0) == SpriteTick(400*time.Millisecond) {
		t.Fatal("moon should advance")
	}
	if MoonGlyph(0, false) != "🌑" {
		t.Fatal(MoonGlyph(0, false))
	}
	if MoonGlyph(4, false) != "🌕" {
		t.Fatal(MoonGlyph(4, false))
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
}

func TestChargeBarFills(t *testing.T) {
	if ChargeBar(2, 8, true) != "[==------]" {
		t.Fatal(ChargeBar(2, 8, true))
	}
}

func TestSpriteHUDIsMoonLoaderLine(t *testing.T) {
	block := SpriteHUD(0, PoseAbacus, "正在读 Codex…  2/6", 2, 6, false, false)
	lines := strings.Split(strings.TrimSuffix(block, "\n"), "\n")
	if len(lines) != 1 {
		t.Fatalf("%#v", lines)
	}
	if !strings.Contains(lines[0], "🌑") || !strings.Contains(lines[0], "拨珠中") || !strings.Contains(lines[0], "正在读 Codex") {
		t.Fatalf("%q", lines[0])
	}
}
