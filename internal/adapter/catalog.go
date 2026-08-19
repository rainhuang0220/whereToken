package adapter

// Catalog is the single tool registry. Scan, doctor, --tool, and labels
// should derive from this list instead of copying IDs across packages.
type Tool struct {
	ID    string
	Label string
	Cloud bool // Cursor / Trae: product usage API, skipped by --offline
}

func Catalog() []Tool {
	return []Tool{
		{ID: "claude", Label: "Claude Code"},
		{ID: "kimi", Label: "Kimi Code"},
		{ID: "grok", Label: "Grok"},
		{ID: "minimax", Label: "MiniMax Agent"},
		{ID: "openclaw", Label: "OpenClaw"},
		{ID: "opencode", Label: "OpenCode"},
		{ID: "codex", Label: "Codex"},
		{ID: "cursor", Label: "Cursor", Cloud: true},
		{ID: "trae", Label: "Trae", Cloud: true},
	}
}

func KnownIDs() []string {
	cat := Catalog()
	out := make([]string, len(cat))
	for i, t := range cat {
		out[i] = t.ID
	}
	return out
}

func Label(id string) string {
	for _, t := range Catalog() {
		if t.ID == id {
			return t.Label
		}
	}
	return id
}
