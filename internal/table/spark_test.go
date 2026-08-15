package table

import "testing"

func TestSparkUnicodeScalesToPeak(t *testing.T) {
	got := Spark([]int64{0, 1, 2, 4, 8, 4, 0}, false)
	if len([]rune(got)) != 7 {
		t.Fatalf("len=%d %q", len([]rune(got)), got)
	}
	runes := []rune(got)
	if runes[0] != '▁' || runes[4] != '█' {
		t.Fatalf("got %q", got)
	}
}

func TestSparkASCIIAllZero(t *testing.T) {
	got := Spark([]int64{0, 0, 0, 0, 0, 0, 0}, true)
	if got != "......." {
		t.Fatalf("got %q", got)
	}
}

func TestSparkEmpty(t *testing.T) {
	if Spark(nil, false) != "" {
		t.Fatal("empty")
	}
}

func TestSparkBlockWidthIsOne(t *testing.T) {
	s := Spark([]int64{1, 2, 3, 4, 5, 6, 7}, false)
	if DisplayWidth(s) != 7 {
		t.Fatalf("width=%d %q", DisplayWidth(s), s)
	}
}
