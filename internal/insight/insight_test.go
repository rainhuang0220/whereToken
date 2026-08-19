package insight

import (
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func TestLinesEmpty(t *testing.T) {
	if n := Lines(metric.Aggregate(nil, nil)); len(n) != 0 {
		t.Fatalf("%v", n)
	}
}

func TestLinesFromCanonicalSummary(t *testing.T) {
	sum := metric.Aggregate([]event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", SessionID: "s1", Miss: 80, CacheRead: 20, Output: 10},
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", SessionID: "s2", Miss: 10, Output: 1},
	}, nil)
	got := Lines(sum)
	blob := ""
	for _, l := range got {
		blob += l.Kind + ":" + l.Text + "\n"
	}
	if !strings.Contains(blob, "largest_tool") || !strings.Contains(blob, "Claude Code") {
		t.Fatalf("%s", blob)
	}
	if !strings.Contains(blob, "缓存读") {
		t.Fatalf("cache %s", blob)
	}
	if !strings.Contains(blob, "API 标价等价") {
		t.Fatalf("cost %s", blob)
	}
	if !strings.Contains(blob, "无标价") || strings.Contains(blob, "$0.0000") {
		t.Fatalf("partial must say unpriced, not invent $0:\n%s", blob)
	}
	if strings.Contains(blob, "Community Rank") || strings.Contains(blob, "global") {
		t.Fatalf("insights must not invent a rank:\n%s", blob)
	}
}

func TestLinesSkipUnlabeledDrillBuckets(t *testing.T) {
	sum := metric.Aggregate([]event.UsageEvent{
		{Source: "minimax", Vendor: "minimax", Miss: 1000},
		{Source: "minimax", Vendor: "minimax", Model: "minimax-m2.5", RequestID: "b", SessionID: "s2", Miss: 10},
	}, nil)
	got := Lines(sum)
	blob := ""
	for _, l := range got {
		blob += l.Kind + ":" + l.Text + "\n"
	}
	if strings.Contains(blob, "(未标模型)") || strings.Contains(blob, "(无会话)") {
		t.Fatalf("unlabeled drill buckets are not a named largest row:\n%s", blob)
	}
	if strings.Contains(blob, "largest_model") {
		t.Fatalf("must not call a smaller labeled model the largest:\n%s", blob)
	}
	if strings.Contains(blob, "largest_session") {
		t.Fatalf("must not call a smaller labeled session the largest:\n%s", blob)
	}
}

func TestAppendStandingNeverZeroRank(t *testing.T) {
	base := []Line{{Kind: "cost", Text: "API 标价等价 · $12.0000（partial）"}}
	if got := AppendStanding(base, "ok", "#0 / 20", 0); len(got) != 1 {
		t.Fatalf("zero podium: %+v", got)
	}
	if got := AppendStanding(base, "ok", "#0 / 20", 1); len(got) != 1 {
		t.Fatalf("hash-zero display: %+v", got)
	}
	if got := AppendStanding(base, "unavailable", "", 0); len(got) != 1 {
		t.Fatalf("unavailable: %+v", got)
	}
	got := AppendStanding(base, "ok", "#37 / 842", 37)
	if len(got) != 2 || got[1].Kind != "community" || !strings.Contains(got[1].Text, "#37 / 842") {
		t.Fatalf("%+v", got)
	}
	if strings.Contains(got[1].Text, "全球") == false {
		t.Fatalf("must say this is not a global rank: %s", got[1].Text)
	}
	if !strings.Contains(got[1].Text, "累计已同步日") {
		t.Fatalf("must say 累计 is uploaded days, not kiln 全部: %s", got[1].Text)
	}
	if strings.Contains(got[1].Text, "#0") {
		t.Fatal(got[1].Text)
	}
}

func TestLinesTinyCostOmitsRoundingZero(t *testing.T) {
	sum := metric.Aggregate([]event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", Miss: 1},
	}, nil)
	got := Lines(sum)
	blob := ""
	for _, l := range got {
		blob += l.Kind + ":" + l.Text + "\n"
	}
	if strings.Contains(blob, "$0.0000") || strings.Contains(blob, "$0.00") {
		t.Fatalf("tiny priced usage must not print a $0 bill:\n%s", blob)
	}
	if strings.Contains(blob, "API 标价等价") {
		t.Fatalf("rounding-to-zero must not be a cost insight:\n%s", blob)
	}
}

func TestLinesUnavailableCostNotZero(t *testing.T) {
	sum := metric.Aggregate([]event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", Miss: 1000, Output: 10},
	}, nil)
	got := Lines(sum)
	blob := ""
	for _, l := range got {
		blob += l.Kind + ":" + l.Text + "\n"
	}
	if !strings.Contains(blob, "估价不可用") {
		t.Fatalf("%s", blob)
	}
	if strings.Contains(blob, "$0.00") || strings.Contains(blob, "cost $0") {
		t.Fatalf("must not write a zero bill:\n%s", blob)
	}
}
