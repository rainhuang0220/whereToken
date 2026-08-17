package table

import (
	"strings"
	"testing"
)

func TestDisplayWidthCJKVsASCII(t *testing.T) {
	cases := []struct {
		in   string
		want int
	}{
		{"", 0},
		{"total", 5},
		{"M", 1},
		{"合计", 4},
		{"命中率", 6},
		{"用户回合", 8},
		{"最长连烧", 8},
		{"Claude Code", 11},
		{"360.11 M", 8},
		{"12,048", 6},
		{"—", 1},
		{"😀", 2},
		{"Ａ", 2},
		{"，", 2},
		{"🚀", 2},
		{"🫶", 2},
		{"한", 2},
	}
	for _, c := range cases {
		if got := DisplayWidth(c.in); got != c.want {
			t.Fatalf("DisplayWidth(%q)=%d want %d", c.in, got, c.want)
		}
	}
}

func TestDisplayWidthIgnoresANSI(t *testing.T) {
	plain := "70.2%"
	painted := "\x1b[32m70.2%\x1b[0m"
	if DisplayWidth(painted) != DisplayWidth(plain) {
		t.Fatalf("ansi width=%d plain=%d", DisplayWidth(painted), DisplayWidth(plain))
	}
	if DisplayWidth("\x1b[1;38;5;208mwhereToken\x1b[0m") != DisplayWidth("whereToken") {
		t.Fatal("title width must ignore kiln color")
	}
}

func TestPadRightUsesDisplayWidth(t *testing.T) {
	got := PadRight("合计", 10)
	if DisplayWidth(got) != 10 {
		t.Fatalf("width=%d %q", DisplayWidth(got), got)
	}
	if got[:len("合计")] != "合计" {
		t.Fatalf("prefix %q", got)
	}
}

func TestPadLeftUsesDisplayWidth(t *testing.T) {
	got := PadLeft("70.2%", 10)
	if DisplayWidth(got) != 10 {
		t.Fatalf("width=%d %q", DisplayWidth(got), got)
	}
	if got[len(got)-5:] != "70.2%" {
		t.Fatalf("suffix %q", got)
	}
}

func TestWrapKeepsShortLine(t *testing.T) {
	got := Wrap("offline · hi", 40)
	if len(got) != 1 || got[0] != "offline · hi" {
		t.Fatalf("%q", got)
	}
}

func TestWrapPrefersIdeographicComma(t *testing.T) {
	s := "offline · 只用本机账本，没有请求 Cursor/Trae 云端"
	got := Wrap(s, 40)
	if len(got) < 2 {
		t.Fatalf("expected wrap: %q", got)
	}
	if !strings.Contains(got[0], "只用本机账本") || !strings.HasSuffix(got[0], "，") {
		t.Fatalf("should break after 账本，: %q", got)
	}
	if strings.HasPrefix(got[1], "Cursor") {
		t.Fatalf("should not orphan Cursor/Trae: %q", got)
	}
}

func TestWrapDoesNotFakeSecondBullet(t *testing.T) {
	s := "  · Unknown 厂家 · 账本没写模型名（Cursor 账号用量常这样）"
	got := Wrap(s, 40)
	for _, line := range got[1:] {
		if strings.HasPrefix(strings.TrimSpace(line), "·") {
			t.Fatalf("fake bullet: %q", got)
		}
	}
}

func TestWrapKeepsEnglishTokenWithCJK(t *testing.T) {
	s := "  · Cursor · token 列不完整（该工具需要已登录）"
	got := Wrap(s, 40)
	if len(got) > 1 && strings.TrimSpace(got[0]) == "· Cursor · token" {
		t.Fatalf("split after token: %q", got)
	}
}

func TestWrapDoesNotBreakAtMiddleDot(t *testing.T) {
	s := "  · Unknown 厂家 · 账本没写模型名（Cursor 账号用量常这样）"
	got := Wrap(s, 40)
	if len(got) > 1 && strings.HasSuffix(strings.TrimSpace(got[0]), "·") {
		t.Fatalf("broke at middle-dot bullet: %q", got)
	}
}

func TestWrapCJKPunctuation(t *testing.T) {
	s := "offline · 只用本机账本，没有请求 Cursor/Trae 云端"
	got := Wrap(s, 40)
	if len(got) < 2 {
		t.Fatalf("expected wrap: %q", got)
	}
	joined := ""
	for i, line := range got {
		if DisplayWidth(line) > 40 {
			t.Fatalf("line %d width %d: %q", i, DisplayWidth(line), line)
		}
		joined += line
	}
	compact := strings.ReplaceAll(strings.ReplaceAll(s, " ", ""), "，", "")
	gotc := strings.ReplaceAll(strings.ReplaceAll(joined, " ", ""), "，", "")
	if !strings.Contains(gotc, "只用本机账本") || !strings.Contains(gotc, "云端") {
		t.Fatalf("lost text: %q vs %q", got, compact)
	}
}

func TestWrapDoesNotSplitASCIIWord(t *testing.T) {
	s := "  · Unknown 厂家 · 账本没写模型名（Cursor 账号用量常这样）"
	got := Wrap(s, 40)
	for _, line := range got {
		if strings.HasSuffix(line, "Curso") || strings.HasPrefix(strings.TrimSpace(line), "r ") {
			t.Fatalf("split Cursor: %q", got)
		}
	}
}
