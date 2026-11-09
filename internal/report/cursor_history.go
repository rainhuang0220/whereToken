package report

import (
	"fmt"
	"strings"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

// cursorHistoryNote describes only the canonical events in the current
// report filters. The retained marker proves no model attribution; a
// normal unknown model is not, by itself, unallocated historical usage.
func cursorHistoryNote(events []event.UsageEvent) string {
	var total int64
	var first, last time.Time
	for _, e := range events {
		if e.Source != "cursor" || strings.TrimSpace(e.Model) != "" || !strings.Contains(e.RequestID, "history-unallocated") {
			continue
		}
		// Match aggregation's saturated addition and exclude reasoning.
		for _, value := range [...]int64{e.Miss, e.CacheRead, e.CacheCreate, e.Output} {
			total = (metric.Slice{Miss: total, Output: value}).Total()
		}
		if e.Timestamp.IsZero() {
			continue
		}
		stamp := e.Timestamp.UTC()
		if first.IsZero() || stamp.Before(first) {
			first = stamp
		}
		if last.IsZero() || stamp.After(last) {
			last = stamp
		}
	}
	if total == 0 {
		return ""
	}
	if first.IsZero() {
		return fmt.Sprintf("Cursor · 历史未分配 %s Token；UTC 记录日未知（非完整覆盖）", metric.FormatCount(total))
	}
	return fmt.Sprintf("Cursor · 历史未分配 %s Token；UTC 记录日 %s … %s（仅有记录日，非完整覆盖）", metric.FormatCount(total), first.Format("2006-01-02"), last.Format("2006-01-02"))
}
