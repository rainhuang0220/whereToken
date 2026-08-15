package cli

import "testing"

func TestSuggestKnownPrefix(t *testing.T) {
	ids := []string{"claude", "kimi", "cursor"}
	if got := suggestKnown("claud", ids); got != "claude" {
		t.Fatalf("got %q", got)
	}
}

func TestSuggestKnownFar(t *testing.T) {
	ids := []string{"claude", "kimi", "cursor"}
	if got := suggestKnown("windsurf", ids); got != "" {
		t.Fatalf("got %q", got)
	}
}
