package ledger

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOpenReadOnlyDoesNotCreateOrMutateLedger(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "ledger.db")
	writer, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := writer.Close(); err != nil {
		t.Fatal(err)
	}
	reader, err := OpenReadOnly(path)
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	if _, err := reader.Meta("schema"); err != nil {
		t.Fatal(err)
	}
	if _, err := reader.db.Exec("DELETE FROM facts"); err == nil {
		t.Fatal("read-only ledger unexpectedly accepted write")
	}
	missing := filepath.Join(dir, "not-created.db")
	if _, err := OpenReadOnly(missing); err == nil {
		t.Fatal("read-only open unexpectedly created missing ledger")
	}
	if _, err := os.Stat(missing); !os.IsNotExist(err) {
		t.Fatalf("missing ledger was created: %v", err)
	}
}
