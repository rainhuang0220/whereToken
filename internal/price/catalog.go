package price

// SourceMeta is the provenance of one vendor's public list-price card: the
// official page the rates were read from and the date a maintainer last
// verified them against that page. Verified is a verification date, not the
// program's run date.
type SourceMeta struct {
	ID       string // matches Rate.Source
	Vendor   string
	URL      string
	Verified string
}

// sources pins every Rate.Source id used by the table to its official
// pricing page. Official vendor documentation only — no aggregators.
var sources = []SourceMeta{
	{"anthropic_api_list", "anthropic", "https://platform.claude.com/docs/en/about-claude/pricing", "2026-08-19"},
	{"xai_api_list", "xai", "https://docs.x.ai/developers/pricing", "2026-08-19"},
	{"openai_api_list", "openai", "https://developers.openai.com/api/docs/pricing", "2026-08-19"},
	{"moonshot_api_list", "moonshot", "https://platform.kimi.ai/docs/pricing", "2026-08-20"},
	{"minimax_api_list", "minimax", "https://platform.minimax.io/docs/guides/pricing-paygo", "2026-08-19"},
	{"zai_api_list", "zhipu", "https://docs.z.ai/guides/overview/pricing", "2026-08-25"},
	{"google_api_list", "google", "https://ai.google.dev/gemini-api/docs/pricing", "2026-08-13"},
	{"deepseek_api_list", "deepseek", "https://api-docs.deepseek.com/quick_start/pricing", "2026-08-25"},
}

// Sources returns provenance for every vendor card, in table order.
func Sources() []SourceMeta { return append([]SourceMeta(nil), sources...) }

// SourceFor resolves a Rate.Source id to its provenance.
func SourceFor(id string) (SourceMeta, bool) {
	for _, s := range sources {
		if s.ID == id {
			return s, true
		}
	}
	return SourceMeta{}, false
}

// Rates returns the baked-in list-price rows in table order. The CLI
// `pricing` catalog and the cost calculator read this same table.
func Rates() []Rate { return append([]Rate(nil), table...) }

// MatchModel reports whether a canonical model id is priced by a row whose
// pattern is pattern — the same boundary rules the calculator uses.
func MatchModel(canon, pattern string) bool { return matchModel(canon, pattern) }
