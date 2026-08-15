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
	case strings.Contains(blob, "deepseek"):
		return "deepseek"
	case strings.Contains(blob, "doubao") || strings.Contains(blob, "seed-code") || strings.Contains(blob, "seed_code") ||
		strings.HasPrefix(m, "seed-"):
		return "doubao"
	case strings.Contains(blob, "glm") || strings.Contains(blob, "zhipu") || strings.Contains(blob, "chatglm"):
		return "zhipu"
	case strings.Contains(blob, "qwen") || strings.Contains(blob, "dashscope"):
		return "alibaba"
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
	case "deepseek":
		return "DeepSeek"
	case "doubao":
		return "Doubao"
	case "zhipu":
		return "Zhipu"
	case "alibaba":
		return "Alibaba"
	default:
		return "Unknown"
	}
}
