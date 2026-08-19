package index

import (
	"fmt"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func fixed(evs []event.UsageEvent) ParseFunc {
	return func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		st, err := f.Stat()
		if err != nil {
			return evs, nil, 0, err
		}
		return evs, nil, st.Size(), nil
	}
}

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
	evs, _, mode, err := store.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		parsed++
		return fixed([]event.UsageEvent{{Source: "claude", RequestID: "r", Miss: 3}})(f)
	})
	if err != nil || mode != ModeFull || parsed != 1 || len(evs) != 1 || evs[0].Miss != 3 {
		t.Fatalf("first %+v mode=%s parsed=%d err=%v", evs, mode, parsed, err)
	}
	evs, _, mode, err = store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		parsed++
		t.Fatal("should not reparse unchanged file")
		return nil, nil, 0, nil
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
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{RequestID: "a", Miss: 1}}))
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

	var start int64
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		st, err := rf.Stat()
		if err != nil {
			return nil, nil, 0, err
		}
		return []event.UsageEvent{{RequestID: "b", Miss: 2}}, nil, st.Size(), nil
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

func TestPartialTailDoesNotAdvanceOffset(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	lineA := "{\"requestId\":\"a\"}\n"
	if err := os.WriteFile(path, []byte(lineA), 0o644); err != nil {
		t.Fatal(err)
	}
	parse := func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		var evs []event.UsageEvent
		consumed, err := ScanJSONL(f, func(line []byte, _ int64) error {
			if len(line) == 0 {
				return nil
			}
			evs = append(evs, event.UsageEvent{RequestID: string(line)})
			return nil
		})
		return evs, nil, consumed, err
	}
	evs, _, _, err := store.LoadOrParse("claude", path, parse)
	if err != nil || len(evs) != 1 {
		t.Fatalf("first %+v err=%v", evs, err)
	}
	if off, err := store.Offset(path); err != nil || off != int64(len(lineA)) {
		t.Fatalf("offset after complete line=%d err=%v", off, err)
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(`{"requestId":"b"`); err != nil {
		t.Fatal(err)
	}
	f.Close()
	now := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, now, now)

	evs, _, mode, err := store.LoadOrParse("claude", path, parse)
	if err != nil || mode != ModeIncremental {
		t.Fatalf("partial mode=%s err=%v", mode, err)
	}
	if len(evs) != 1 || evs[0].RequestID != lineA[:len(lineA)-1] {
		t.Fatalf("partial must keep only a: %+v", evs)
	}
	if off, err := store.Offset(path); err != nil || off != int64(len(lineA)) {
		t.Fatalf("partial must not advance offset: %d err=%v", off, err)
	}

	f, err = os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString("}\n"); err != nil {
		t.Fatal(err)
	}
	f.Close()
	later := time.Now().Add(4 * time.Second)
	_ = os.Chtimes(path, later, later)

	evs, _, mode, err = store.LoadOrParse("claude", path, parse)
	if err != nil || mode != ModeIncremental {
		t.Fatalf("complete mode=%s err=%v", mode, err)
	}
	if len(evs) != 2 {
		t.Fatalf("completed line must appear: %+v", evs)
	}
	if evs[1].RequestID != `{"requestId":"b"}` {
		t.Fatalf("second=%q", evs[1].RequestID)
	}
	st, err := os.Stat(path)
	if err != nil {
		t.Fatal(err)
	}
	if off, err := store.Offset(path); err != nil || off != st.Size() {
		t.Fatalf("after complete offset=%d size=%d err=%v", off, st.Size(), err)
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
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{RequestID: "old", Miss: 9}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var start int64 = -1
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		return fixed([]event.UsageEvent{{RequestID: "new", Miss: 1}})(rf)
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
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{RequestID: "old"}}))
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
	evs, _, mode, err := store.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		parsed++
		return fixed([]event.UsageEvent{{RequestID: "new"}})(f)
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
	_, _, _, err = store.LoadOrReplay("codex", path, fixed([]event.UsageEvent{{RequestID: "a"}}))
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
	_, _, mode, err := store.LoadOrReplay("codex", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		return fixed([]event.UsageEvent{{RequestID: "b"}})(rf)
	})
	if err != nil || mode != ModeFull || start != 0 {
		t.Fatalf("replay must full-rescan appends: mode=%s start=%d err=%v", mode, start, err)
	}
}

func TestIncrementalParseErrorKeepsOldEvents(t *testing.T) {
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
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{RequestID: "old", Miss: 9}}))
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
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)

	evs, _, mode, err := store.LoadOrParse("claude", path, func(_ *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		return nil, nil, 0, fmt.Errorf("jsonl line exceeds limit")
	})
	if err == nil {
		t.Fatal("want parse error")
	}
	if mode != ModeIncremental {
		t.Fatalf("mode=%s", mode)
	}
	if len(evs) != 1 || evs[0].RequestID != "old" || evs[0].Miss != 9 {
		t.Fatalf("incremental parse error must keep cached events: %+v", evs)
	}
}

func TestSameSizeMtimeChangeIsFullRescan(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("same-size\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{RequestID: "old", Miss: 9}}))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("rewritten\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)
	parsed := 0
	var start int64 = -1
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		parsed++
		off, _ := rf.Seek(0, 1)
		start = off
		return fixed([]event.UsageEvent{{RequestID: "new", Miss: 1}})(rf)
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeFull || parsed != 1 || start != 0 || evs[0].RequestID != "new" {
		t.Fatalf("same-size rewrite must full-rescan: mode=%s parsed=%d start=%d evs=%+v", mode, parsed, start, evs)
	}
}

func TestSameSizeRewriteWithPendingTailIsFullRescan(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	path := filepath.Join(dir, "a.jsonl")
	if err := os.WriteFile(path, []byte("keep-a\npartial"), 0o644); err != nil {
		t.Fatal(err)
	}
	_, _, _, err = store.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		n, err := ScanJSONL(f, func([]byte, int64) error { return nil })
		return []event.UsageEvent{{RequestID: "old", Miss: 9}}, nil, n, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("rewrote\nxxxxx"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)
	var start int64 = -1
	evs, _, mode, err := store.LoadOrParse("claude", path, func(rf *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		off, _ := rf.Seek(0, 1)
		start = off
		n, err := ScanJSONL(rf, func([]byte, int64) error { return nil })
		return []event.UsageEvent{{RequestID: "new", Miss: 1}}, nil, n, err
	})
	if err != nil {
		t.Fatal(err)
	}
	if mode != ModeFull || start != 0 || evs[0].RequestID != "new" {
		t.Fatalf("pending-tail rewrite must full-rescan: mode=%s start=%d evs=%+v", mode, start, evs)
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
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{Miss: 1}}))
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
	_, _, mode, err := store.LoadOrParse("claude", path, func(f *os.File) ([]event.UsageEvent, []event.TurnEvent, int64, error) {
		parsed++
		return fixed([]event.UsageEvent{{Miss: 1}})(f)
	})
	if err != nil || mode != ModeFull || parsed != 1 {
		t.Fatalf("after wipe mode=%s parsed=%d err=%v", mode, parsed, err)
	}
}
