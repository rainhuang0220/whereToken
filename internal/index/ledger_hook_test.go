package index

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

type rejectFacts struct{ called int }

func (r *rejectFacts) CommitUsage([]event.UsageEvent) error {
	r.called++
	return errors.New("ledger rejected")
}

type orderFacts struct {
	store *Store
	early bool
}

func (o *orderFacts) CommitUsage([]event.UsageEvent) error {
	var n int
	if err := o.store.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&n); err != nil {
		return err
	}
	o.early = n == 0
	return nil
}

func TestCheckpointWaitsForTheLedger(t *testing.T) {
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
	rej := &rejectFacts{}
	store.SetFacts(rej)
	_, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{Source: "claude", RequestID: "r", Miss: 3}}))
	if err == nil || rej.called != 1 {
		t.Fatalf("err=%v called=%d", err, rej.called)
	}
	var n int
	if err := store.db.QueryRow(`SELECT COUNT(*) FROM files`).Scan(&n); err != nil || n != 0 {
		t.Fatalf("checkpoint moved files=%d err=%v", n, err)
	}
	ord := &orderFacts{store: store}
	store.SetFacts(ord)
	if _, _, _, err = store.LoadOrParse("claude", path, fixed([]event.UsageEvent{{Source: "claude", RequestID: "r", Miss: 3}})); err != nil {
		t.Fatal(err)
	}
	if !ord.early {
		t.Fatal("checkpoint was visible before the ledger commit returned")
	}
}

func TestBackupAndExportKeepDeletedFiles(t *testing.T) {
	dir := t.TempDir()
	store, err := Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "gone.jsonl")
	if err := os.WriteFile(path, []byte("x\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	ev := event.UsageEvent{Source: "codex", SessionID: "sess", RequestID: path + ":1", Miss: 8, Output: 2}
	if _, _, _, err = store.LoadOrParse("codex", path, fixed([]event.UsageEvent{ev})); err != nil {
		t.Fatal(err)
	}
	if err := os.Remove(path); err != nil {
		t.Fatal(err)
	}
	dest := filepath.Join(dir, "back", "idx.bak")
	if err := store.BackupTo(dest); err != nil {
		t.Fatal(err)
	}
	got, gaps, err := store.ExportUsage()
	if err != nil || gaps != 0 || len(got) != 1 || got[0].Miss != 8 {
		t.Fatalf("export %+v gaps=%d err=%v", got, gaps, err)
	}
	store.Close()
	if err := Wipe(filepath.Join(dir, "idx.db")); err != nil {
		t.Fatal(err)
	}
	opened, err := Open(dest)
	if err != nil {
		t.Fatal(err)
	}
	defer opened.Close()
	got, gaps, err = opened.ExportUsage()
	if err != nil || gaps != 0 || len(got) != 1 || got[0].Miss != 8 || got[0].Output != 2 {
		t.Fatalf("backup export %+v gaps=%d err=%v", got, gaps, err)
	}
}
