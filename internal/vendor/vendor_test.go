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
		{"DeepSeek-V4-Flash", "", "deepseek"},
		{"Doubao-Seed-2.0-Code", "", "doubao"},
		{"Seed-Code", "", "doubao"},
		{"glm-5.2", "", "zhipu"},
		{"qwen-3.7-plus", "", "alibaba"},
	}
	for _, c := range cases {
		if got := Lookup(c.model, c.provider); got != c.want {
			t.Fatalf("Lookup(%q,%q)=%q want %q", c.model, c.provider, got, c.want)
		}
	}
}

func TestLookupName(t *testing.T) {
	cases := []struct{ in, want string }{
		{"anthropic", "anthropic"},
		{"Anthropic", "anthropic"},
		{"minimax", "minimax"},
		{"MiniMax", "minimax"},
		{"moonshot", "moonshot"},
		{"Moonshot", "moonshot"},
		{"openai", "openai"},
		{"OpenAI", "openai"},
		{"google", "google"},
		{"deepseek", "deepseek"},
		{"DeepSeek", "deepseek"},
		{"doubao", "doubao"},
		{"zhipu", "zhipu"},
		{"Zhipu", "zhipu"},
		{"alibaba", "alibaba"},
		{"unknown", "unknown"},
		{"Unknown", "unknown"},
		{"未知厂家", "unknown"},
	}
	for _, c := range cases {
		got, ok := LookupName(c.in)
		if !ok || got != c.want {
			t.Fatalf("LookupName(%q)=%q %v want %q true", c.in, got, ok, c.want)
		}
	}
	if _, ok := LookupName("windsurf"); ok {
		t.Fatal("windsurf is not a vendor")
	}
	if _, ok := LookupName(""); ok {
		t.Fatal("empty")
	}
}

func TestLabel(t *testing.T) {
	if Label("moonshot") != "Moonshot" {
		t.Fatal(Label("moonshot"))
	}
	if Label("unknown") != "未知厂家" {
		t.Fatal(Label("unknown"))
	}
	if Label("deepseek") != "DeepSeek" {
		t.Fatal(Label("deepseek"))
	}
	if Label("doubao") != "Doubao" {
		t.Fatal(Label("doubao"))
	}
}
