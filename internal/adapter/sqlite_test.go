package adapter

import (
	"database/sql"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"
)

func TestSQLiteURIKeepsModeQueryAndEscapesPathMarks(t *testing.T) {
	u := SQLiteURI("/tmp/foo?bar.db", "mode=ro")
	if strings.Contains(u, "foo?bar") {
		t.Fatalf("bare ? in path would steal the query: %s", u)
	}
	if !strings.Contains(u, "%3F") {
		t.Fatalf("expected escaped ?: %s", u)
	}
	if !strings.Contains(u, "mode=ro") {
		t.Fatalf("mode query dropped: %s", u)
	}
	if !strings.HasPrefix(u, "file:") {
		t.Fatalf("not a file URI: %s", u)
	}
}

func TestSQLiteURIEscapesHash(t *testing.T) {
	u := SQLiteURI("/tmp/a#b.db", "mode=ro&immutable=1")
	if strings.Contains(u, "a#b") {
		t.Fatalf("bare # would start a fragment: %s", u)
	}
	if !strings.Contains(u, "immutable=1") {
		t.Fatalf("query dropped: %s", u)
	}
}

func TestOpenROReadsExistingFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "ledger.db")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE t (id INTEGER)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	ro, err := OpenRO(path)
	if err != nil {
		t.Fatal(err)
	}
	defer ro.Close()
	var n int
	if err := ro.QueryRow(`SELECT count(*) FROM t`).Scan(&n); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("count=%d", n)
	}
}
