package card

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestFromSnapshotKeepsTopThreeAndAggregatesRest(t *testing.T) {
	value := func(n int64) publicprofile.Totals {
		return publicprofile.Totals{Total: publicprofile.Component{Value: &n, Display: "n", Status: publicprofile.StatusAvailable}}
	}
	rows := []publicprofile.Breakdown{
		{ID: "claude", Label: "Claude", Totals: value(50), Share: "50%"},
		{ID: "codex", Label: "Codex", Totals: value(20), Share: "20%"},
		{ID: "cursor", Label: "Cursor", Totals: value(15), Share: "15%"},
		{ID: "grok", Label: "Grok", Totals: value(10), Share: "10%"},
		{ID: "kimi", Label: "Kimi", Totals: value(5), Share: "5%"},
	}
	snap := publicprofile.Snapshot{Periods: publicprofile.Periods{All: publicprofile.Period{Totals: value(100), ByAgent: rows}}}
	got := FromSnapshot(snap).Agents
	if len(got) != 4 || !got[3].Rest || got[3].TokensRaw != 15 || got[3].ShareText != "15.0%" {
		t.Fatalf("agents=%+v", got)
	}
}
