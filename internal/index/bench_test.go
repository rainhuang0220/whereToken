package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func BenchmarkFullThenUnchanged(b *testing.B) {
	dir := b.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "big.jsonl")
	f, err := os.Create(path)
	if err != nil {
		b.Fatal(err)
	}
	for i := 0; i < 8000; i++ {
		fmt.Fprintf(f, "{\"n\":%d}\n", i)
	}
	f.Close()

	parse := func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		evs := make([]event.UsageEvent, 8000)
		for i := range evs {
			evs[i] = event.UsageEvent{Source: "claude", RequestID: fmt.Sprintf("r%d", i), Miss: 1}
		}
		return evs, nil, nil
	}
	if _, _, _, err := store.LoadOrParse("claude", path, parse); err != nil {
		b.Fatal(err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		evs, _, mode, err := store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
			b.Fatal("unchanged file must not reparse")
			return nil, nil, nil
		})
		if err != nil || mode != ModeUnchanged || len(evs) != 8000 {
			b.Fatalf("mode=%s n=%d err=%v", mode, len(evs), err)
		}
	}
}
