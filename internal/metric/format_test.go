package metric

import "testing"

func TestFormatM(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0.00 M"},
		{1_000_000, "1.00 M"},
		{360_109_885, "360.11 M"},
		{1_741, "0.0017 M"},
		{9_999, "0.0100 M"},
		{10_000, "0.01 M"},
		{1_000_000_000_000, "1000000.00 M"},
	}
	for _, c := range cases {
		if got := FormatM(c.in); got != c.want {
			t.Fatalf("FormatM(%d)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestFormatCount(t *testing.T) {
	cases := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{12, "12"},
		{999, "999"},
		{1000, "1,000"},
		{12048, "12,048"},
		{1_000_000, "1,000,000"},
		{9_223_372_036_854_775_807, "9,223,372,036,854,775,807"},
	}
	for _, c := range cases {
		if got := FormatCount(c.in); got != c.want {
			t.Fatalf("FormatCount(%d)=%q want %q", c.in, got, c.want)
		}
	}
}

func TestHitRate(t *testing.T) {
	pct, ok := HitRate(97_763_998, 252_128_914, 8_952_459)
	if !ok {
		t.Fatal("expected ok")
	}
	if pct < 70.2 || pct > 70.3 {
		t.Fatalf("pct=%v", pct)
	}
	if _, ok := HitRate(0, 0, 0); ok {
		t.Fatal("zero denom must not be ok")
	}
}
