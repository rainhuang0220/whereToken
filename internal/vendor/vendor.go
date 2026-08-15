package vendor

import "strings"

func Lookup(model, provider string) string {
	m := strings.ToLower(strings.TrimSpace(model))
	p := strings.ToLower(strings.TrimSpace(provider))
	blob := strings.TrimSpace(m + " " + p)

	switch {
	case strings.Contains(blob, "minimax") || strings.HasPrefix(m, "abab"):
		return "minimax"
	case strings.Contains(blob, "claude"):
		return "anthropic"
	case strings.Contains(blob, "kimi") || strings.Contains(blob, "moonshot") || m == "k3":
		return "moonshot"
	case strings.Contains(blob, "gpt") || strings.Contains(blob, "chatgpt") || strings.Contains(m, "codex") ||
		hasOpenAIReasoningPrefix(m):
		return "openai"
	case strings.Contains(blob, "gemini"):
		return "google"
	default:
		return "unknown"
	}
}

func hasOpenAIReasoningPrefix(model string) bool {
	for _, p := range []string{"o1", "o3", "o4"} {
		if model == p || strings.HasPrefix(model, p+"-") || strings.HasPrefix(model, p+".") {
			return true
		}
	}
	return false
}

func Label(id string) string {
	switch id {
	case "anthropic":
		return "Anthropic"
	case "moonshot":
		return "Moonshot"
	case "openai":
		return "OpenAI"
	case "minimax":
		return "MiniMax"
	case "google":
		return "Google"
	default:
		return "Unknown"
	}
}
