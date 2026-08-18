package index

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestUnchangedFileReplaysWithoutParse(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("{\"n\":1}\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed := 0
	evs, _, mode, err := store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		parsed++
		return []event.UsageEvent{{Source: "claude", RequestID: "r", Miss: 3}}, nil, nil
	})
	if err != nil || mode != ModeFull || parsed != 1 || len(evs) != 1 || evs[0].Miss != 3 {
		t.Fatalf("first %+v mode=%s parsed=%d err=%v", evs, mode, parsed, err)
	}
	evs, _, mode, err = store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		parsed++
		t.Fatal("should not reparse unchanged file")
		return nil, nil, nil
	})
	if err != nil || mode != ModeUnchanged || parsed != 1 || evs[0].Miss != 3 {
		t.Fatalf("second %+v mode=%s parsed=%d err=%v", evs, mode, parsed, err)
	}
}

func TestAppendOnlyReadsNewBytes(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		return []event.UsageEvent{{RequestID: "a", Miss: 1}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	// bump mtime so some filesystems don't keep the same stamp
	now := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, now, now)

	var start int64
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		return []event.UsageEvent{{RequestID: "b", Miss: 2}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeIncremental {
		t.Fatalf("mode=%s", mode)
	}
	if start != 6 { // "line1\n"
		t.Fatalf("offset=%d want 6", start)
	}
	if len(evs) != 2 || evs[0].RequestID != "a" || evs[1].RequestID != "b" {
		t.Fatalf("merged %+v", evs)
	}
}

func TestTruncateForcesFullRescan(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("0123456789\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		return []event.UsageEvent{{RequestID: "old", Miss: 9}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var start int64 = -1
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		return []event.UsageEvent{{RequestID: "new", Miss: 1}}, nil, nil
	})
	if err != nil || mode != ModeFull || start != 0 || evs[0].RequestID != "new" {
		t.Fatalf("mode=%s start=%d evs=%+v err=%v", mode, start, evs, err)
	}
}

func TestReplaceSameSizeDifferentInodeForcesFull(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("same-bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		return []event.UsageEvent{{RequestID: "old"}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("same-bytes\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	parsed := 0
	evs, _, mode, err := store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		parsed++
		return []event.UsageEvent{{RequestID: "new"}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inodeOf(mustStat(t, path)) == 0 {
		if mode != ModeUnchanged && mode != ModeFull {
			t.Fatalf("no inode: mode=%s", mode)
		}
		return
	}
	if mode != ModeFull || parsed != 1 || evs[0].RequestID != "new" {
		t.Fatalf("replaced file mode=%s parsed=%d evs=%+v", mode, parsed, evs)
	}
}

func mustStat(t *testing.T, path string) os.FileInfo {
	t.Helper()
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	return st
}

func TestLoadOrReplayNeverSeeksMidFile(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("line1\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrReplay("codex", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		return []event.UsageEvent{{RequestID: "a"}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("line2\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	now := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, now, now)
	var start int64 = -1
	_, _, mode, err := store.LoadOrReplay("codex", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		return []event.UsageEvent{{RequestID: "b"}}, nil, nil
	})
	if err != nil || mode != ModeFull || start != 0 {
		t.Fatalf("replay must full-rescan appends: mode=%s start=%d err=%v", mode, start, err)
	}
}

func TestWipeClearsIndex(t *testing.T) {
	dir := t.TempDir()
	p := filepath.Join(dir, "idx.db")
	store, err := Open(p)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("a\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		return []event.UsageEvent{{Miss: 1}}, nil, nil
	})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	if err := Wipe(p); err != nil {
		t.Fatal(err)
	}
	store, err = Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	parsed := 0
	_, _, mode, err := store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, error) {
		parsed++
		return []event.UsageEvent{{Miss: 1}}, nil, nil
	})
	if err != nil || mode != ModeFull || parsed != 1 {
		t.Fatalf("after wipe mode=%s parsed=%d err=%v", mode, parsed, err)
	}
}
