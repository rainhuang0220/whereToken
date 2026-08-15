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
