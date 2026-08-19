package metric

import (
	"fmt"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

const benchEventCount = 4000

func syntheticUploadEvents(n int) []event.UsageEvent {
	sources := [...]string{"claude", "kimi", "codex", "cursor", "trae"}
	vendors := [...]string{"anthropic", "moonshot", "openai", "anthropic", "unknown"}
	models := [...]string{"claude-opus-4.6", "k2.5", "gpt-5", "claude-sonnet-4.6", "k3"}
	base := time.Date(2026, 8, 1, 12, 0, 0, 0, time.UTC)
	evs := make([]event.UsageEvent, n)
	for i := 0; i < n; i++ {
		k := i % len(sources)
		// Repeat some request ids so mergeByRequest does work.
		rid := i / 3
		evs[i] = event.UsageEvent{
			Source:      sources[k],
			Vendor:      vendors[k],
			Model:       models[k],
			RequestID:   fmt.Sprintf("r%d", rid),
			Timestamp:   base.Add(time.Duration(i) * time.Hour),
			Miss:        int64(1000 + i%500),
			CacheRead:   int64(i % 800),
			CacheCreate: int64(i % 40),
			Output:      int64(20 + i%80),
		}
	}
	return evs
}

// BenchmarkCostSliceVsAggregate is a smoke bench for the community upload path.
// CostSlice skips calendar and drill; do not gate CI on ns/op.
func BenchmarkCostSliceVsAggregate(b *testing.B) {
	events := syntheticUploadEvents(benchEventCount)
	b.Run("CostSlice", func(b *testing.B) {
		b.ReportAllocs()
		var s Slice
		for i := 0; i < b.N; i++ {
			s = CostSlice(events)
		}
		if s.Total() == 0 {
			b.Fatal("empty CostSlice")
		}
	})
	b.Run("Aggregate", func(b *testing.B) {
		b.ReportAllocs()
		var sum Summary
		for i := 0; i < b.N; i++ {
			sum = Aggregate(events, nil)
		}
		if sum.All.Total() == 0 {
			b.Fatal("empty Aggregate")
		}
	})
}
