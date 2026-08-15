package fuzzy

import "testing"

func TestDistance(t *testing.T) {
	cases := []struct {
		a, b string
		d    int
	}{
		{"", "", 0},
		{"", "ab", 2},
		{"same", "same", 0},
		{"k3", "k4", 1},
		{"claude", "claud", 1},
		{"anthropic", "anthropc", 1},
		{"猫", "狗", 1},
	}
	for _, c := range cases {
		if got := Distance(c.a, c.b); got != c.d {
			t.Fatalf("Distance(%q,%q)=%d want %d", c.a, c.b, got, c.d)
		}
	}
}

func TestClosest(t *testing.T) {
	ids := []string{"claude", "kimi", "cursor"}
	if got := Closest("claud", ids, 2); got != "claude" {
		t.Fatalf("got %q", got)
	}
	if got := Closest("windsurf", ids, 2); got != "" {
		t.Fatalf("got %q", got)
	}
}
