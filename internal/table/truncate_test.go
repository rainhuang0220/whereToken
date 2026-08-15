package table

import "testing"

func TestTruncateASCIIAndCJK(t *testing.T) {
	if got := Truncate("hello", 10); got != "hello" {
		t.Fatal(got)
	}
	got := Truncate("abcdefghijklmnopqrstuvwxyz", 8)
	if DisplayWidth(got) != 8 {
		t.Fatalf("width=%d %q", DisplayWidth(got), got)
	}
	if !hasEllipsis(got) {
		t.Fatalf("expected ellipsis %q", got)
	}
	got = Truncate("用户回合用户回合用户回合", 8)
	if DisplayWidth(got) > 8 {
		t.Fatalf("cjk width=%d %q", DisplayWidth(got), got)
	}
}

func hasEllipsis(s string) bool {
	for _, r := range s {
		if r == '…' {
			return true
		}
	}
	return false
}
