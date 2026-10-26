package index

import (
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestAbsentUnderKeepsPrefixAndSkipsCorrupt(t *testing.T) {
	dir := t.TempDir()
	st, err := Open(filepath.Join(dir, "index.v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	root := filepath.Join(dir, "codex")
	keep := filepath.Join(root, "sessions", "2026", "10", "01", "rollout-keep.jsonl")
	other := filepath.Join(dir, "codex-other", "sessions", "2026", "10", "01", "rollout-other.jsonl")
	row := fileRow{Size: 1, Mtime: 1, Inode: 1, Offset: 1}
	if err := st.put(keep, row, 1, []event.UsageEvent{{Source: "codex", SessionID: "rollout-keep", Miss: 10}}, nil); err != nil {
		t.Fatal(err)
	}
	if err := st.put(other, row, 1, []event.UsageEvent{{Source: "codex", SessionID: "rollout-other", Miss: 99}}, nil); err != nil {
		t.Fatal(err)
	}
	corrupt := filepath.Join(root, "sessions", "2026", "10", "01", "rollout-bad.jsonl")
	if _, err := st.db.Exec(`INSERT INTO blobs(path, events, turns) VALUES(?, ?, ?)`, corrupt, []byte("not-a-blob"), []byte{}); err != nil {
		t.Fatal(err)
	}
	got := st.AbsentUnder(root)
	if len(got) != 1 || got[0].Path != keep || len(got[0].Events) != 1 || got[0].Events[0].Miss != 10 {
		t.Fatalf("absent %+v", got)
	}
	if AbsentUnder(root) != nil {
		t.Fatal("no active store must not invent blobs")
	}
}
