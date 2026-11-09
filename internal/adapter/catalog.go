package adapter

// Catalog is the single tool registry. Scan, doctor, --tool, and labels
// should derive from this list instead of copying IDs across packages.
type Tool struct {
	ID    string
	Label string
	Cloud bool // Cursor / Trae: product usage API, skipped by --offline
	Caps  Caps
}

func localLedgerCaps(archive, incremental bool) Caps {
	c := Caps{
		Discovery: LevelYes, Usage: LevelYes, Cost: LevelUnavailable,
		Model: LevelYes, Timestamp: LevelYes, Session: LevelYes,
		Cache: LevelYes, Reasoning: LevelUnknown, Archive: LevelUnavailable,
		Incremental: LevelUnavailable,
	}
	if archive {
		c.Archive = LevelYes
	}
	if incremental {
		c.Incremental = LevelYes
	}
	return c
}

func Catalog() []Tool {
	jsonl := localLedgerCaps(false, true)
	jsonl.Reasoning = LevelYes
	openclaw := localLedgerCaps(true, true)
	sqlite := localLedgerCaps(false, false)
	sqliteReasoning := sqlite
	sqliteReasoning.Reasoning = LevelYes
	cloud := Caps{
		Discovery: LevelYes, Usage: LevelYes, Cost: LevelUnavailable,
		Model: LevelYes, Timestamp: LevelYes, Session: LevelYes,
		Cache: LevelYes, Incremental: LevelUnavailable,
	}
	return []Tool{
		{ID: "claude", Label: "Claude Code", Caps: jsonl},
		{ID: "kimi", Label: "Kimi Code", Caps: jsonl},
		{ID: "grok", Label: "Grok", Caps: jsonl},
		{ID: "minimax", Label: "MiniMax Agent", Caps: sqlite},
		{ID: "openclaw", Label: "OpenClaw", Caps: openclaw},
		{ID: "opencode", Label: "OpenCode", Caps: sqlite},
		{ID: "codex", Label: "Codex", Caps: localLedgerCaps(true, false)},
		{ID: "cursor", Label: "Cursor", Cloud: true, Caps: cloud},
		{ID: "trae", Label: "Trae", Cloud: true, Caps: Caps{Discovery: LevelYes, Usage: LevelUnavailable}},
		{ID: "gemini", Label: "Gemini CLI", Caps: jsonl},
		{ID: "qwen", Label: "Qwen Code", Caps: jsonl},
		{ID: "cline", Label: "Cline", Caps: localLedgerCaps(false, false)},
		{ID: "roo", Label: "Roo Code", Caps: localLedgerCaps(false, false)},
		{ID: "kilo", Label: "Kilo Code", Caps: localLedgerCaps(false, false)},
		{ID: "zcode", Label: "ZCode", Caps: sqliteReasoning},
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

// Label is called on every normalized usage event, potentially millions of
// times during a Profile projection. Cache the fixed catalog labels instead
// of allocating all capability structs on every call.
var labelByID = func() map[string]string {
	out := map[string]string{}
	for _, t := range Catalog() {
		out[t.ID] = t.Label
	}
	return out
}()

func Label(id string) string {
	if name, ok := labelByID[id]; ok {
		return name
	}
	return id
}
