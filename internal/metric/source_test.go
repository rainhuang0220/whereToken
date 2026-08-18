package metric

import "testing"

func TestLookupSourceAcceptsIDsAndLabels(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{"claude", "claude"},
		{"Claude Code", "claude"},
		{"CLAUDE-CODE", "claude"},
		{"kimi", "kimi"},
		{"Kimi Code", "kimi"},
		{"codex", "codex"},
		{"opencode", "opencode"},
		{"OpenCode", "opencode"},
		{"cursor", "cursor"},
		{"trae", "trae"},
		{"TRAE", "trae"},
		{"grok", "grok"},
		{"Grok", "grok"},
		{"minimax", "minimax"},
		{"MiniMax Agent", "minimax"},
	}
	for _, c := range cases {
		got, ok := LookupSource(c.in)
		if !ok || got != c.want {
			t.Fatalf("LookupSource(%q)=%q %v want %q true", c.in, got, ok, c.want)
		}
	}
}

func TestLookupSourceRejectsUnknown(t *testing.T) {
	if _, ok := LookupSource("windsurf"); ok {
		t.Fatal("windsurf must be unknown")
	}
	if _, ok := LookupSource(""); ok {
		t.Fatal("empty must be unknown")
	}
}
