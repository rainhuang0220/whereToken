package report

import (
	"strings"
	"testing"
)

func TestSuggestModelSubstring(t *testing.T) {
	events, _ := fixture(shanghai())
	got := suggestModel("opus", events)
	if !strings.Contains(strings.ToLower(got), "opus") {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestModelTypo(t *testing.T) {
	events, _ := fixture(shanghai())
	got := suggestModel("k4", events)
	if got != "k3" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestModelNoMatch(t *testing.T) {
	events, _ := fixture(shanghai())
	if got := suggestModel("nope-model", events); got != "" {
		t.Fatalf("got %q", got)
	}
}
