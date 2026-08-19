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
	if !strings.Contains(blob, "Cache Read") {
		t.Fatalf("cache %s", blob)
	}
	if !strings.Contains(blob, "API-equivalent") {
		t.Fatalf("cost %s", blob)
	}
	if !strings.Contains(blob, "Unpriced") || strings.Contains(blob, "$0") && strings.Contains(blob, "k3") && !strings.Contains(blob, "not $0") {
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

func TestLinesUnavailableCostNotZero(t *testing.T) {
	sum := metric.Aggregate([]event.UsageEvent{
		{Source: "kimi", Vendor: "moonshot", Model: "k3", RequestID: "b", Miss: 1000, Output: 10},
	}, nil)
	got := Lines(sum)
	blob := ""
	for _, l := range got {
		blob += l.Kind + ":" + l.Text + "\n"
	}
	if !strings.Contains(blob, "unavailable") {
		t.Fatalf("%s", blob)
	}
	if strings.Contains(blob, "$0.00") || strings.Contains(blob, "cost $0") {
		t.Fatalf("must not write a zero bill:\n%s", blob)
	}
}
