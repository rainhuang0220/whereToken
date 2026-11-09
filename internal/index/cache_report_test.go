package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestReadOnlyCacheExportsUsageAndTurns(t *testing.T) {
	dir := t.TempDir()
	indexPath := filepath.Join(dir, "index.db")
	source := filepath.Join(dir, "source.jsonl")
	if err := os.WriteFile(source, []byte("{}\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	writer, err := Open(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	when := time.Date(2026, 11, 7, 15, 0, 0, 0, time.UTC)
	ev := event.UsageEvent{Source: "claude", Vendor: "anthropic", Model: "claude-opus-4.6", RequestID: "a", SessionID: "s", Timestamp: when, Miss: 42}
	turn := event.TurnEvent{Source: "claude", SessionID: "s", Timestamp: when}
	_, _, _, err = writer.LoadOrReplay("claude", source, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		st, err := f.Stat()
		if err != nil {
			return nil, nil, 0, err
		}
		return []event.UsageEvent{ev}, []event.TurnEvent{turn}, st.Size(), nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnly(indexPath)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	events, turns, gaps, err := reader.ExportCached()
	if err != nil || gaps != 0 || len(events) != 1 || len(turns) != 1 ||
		events[0].Miss != 42 || turns[0].SessionID != "s" {
		t.Fatalf("events=%+v turns=%+v gaps=%d err=%v", events, turns, gaps, err)
	}
	if _, err := reader.db.Exec("DELETE FROM blobs"); err == nil {
		t.Fatal("read-only cache unexpectedly accepted a write")
	}
	if _, err := OpenReadOnly(filepath.Join(dir, "nonexistent.db")); err == nil {
		t.Fatal("read-only open unexpectedly created a missing cache")
	}
	if _, err := os.Stat(filepath.Join(dir, "nonexistent.db")); !os.IsNotExist(err) {
		t.Fatalf("read-only open created a missing cache: %v", err)
	}
}
