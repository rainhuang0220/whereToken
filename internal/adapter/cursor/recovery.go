package cursor

import (
	"context"
	"time"
)

// scheduledWindows starts with the most recent UTC span and, when needed,
// the preceding month. Up to one older uncovered window rotates through the
// remaining dates across watch slots. The caller still reports the complete
// lookback so the skipped dates remain visible as gaps.
func scheduledWindows(windows, all []usageSpan, now time.Time) []usageSpan {
	if len(windows) < 2 {
		return windows
	}
	selected := []usageSpan{windows[len(windows)-1]}
	older := windows[:len(windows)-1]
	if len(all) >= 2 && older[len(older)-1].start.Equal(all[len(all)-2].start) {
		selected = append(selected, older[len(older)-1])
		older = older[:len(older)-1]
	}
	if len(older) > 0 {
		utc := now.UTC()
		// The stride is prime and greater than the number of months in
		// the lookback. Even a fixed daily check can visit every gap.
		slot := utc.Unix()/86400*17 + int64(utc.Hour()/8)
		index := (slot%int64(len(older)) + int64(len(older))) % int64(len(older))
		selected = append(selected, older[index])
	}
	return selected
}

func (a Adapter) requestContext() context.Context {
	if a.collectionContext != nil {
		return a.collectionContext
	}
	return context.Background()
}
