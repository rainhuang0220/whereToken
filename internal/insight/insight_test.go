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
}
