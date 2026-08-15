package vendor

import "testing"

func TestLookup(t *testing.T) {
	cases := []struct{ model, provider, want string }{
		{"MiniMax-M3", "", "minimax"},
		{"claude-opus-4.6", "", "anthropic"},
		{"kimi-code/k3", "", "moonshot"},
		{"k3", "kimi-for-coding", "moonshot"},
		{"gpt-5", "", "openai"},
		{"o3-mini", "", "openai"},
		{"gemini-2.5-pro", "", "google"},
		{"totally-unknown-model", "", "unknown"},
	}
	for _, c := range cases {
		if got := Lookup(c.model, c.provider); got != c.want {
			t.Fatalf("Lookup(%q,%q)=%q want %q", c.model, c.provider, got, c.want)
		}
	}
}

func TestLabel(t *testing.T) {
	if Label("moonshot") != "Moonshot" {
		t.Fatal(Label("moonshot"))
	}
	if Label("unknown") != "Unknown" {
		t.Fatal(Label("unknown"))
	}
}
