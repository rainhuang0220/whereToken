package opencode

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestParseMessageTokens(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
		"u1", "s1", 1, 1, `{"role":"user"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
		"a1", "s1", 2, 2, `{"role":"assistant","tokens":{"input":100,"output":10,"reasoning":2,"cache":{"read":50,"write":5}},"modelID":"k3","providerID":"kimi-for-coding"}`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var evs []event.UsageEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "opencode", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	got := evs[0]
	if got.Miss != 100 || got.Output != 12 || got.Reasoning != 2 {
		t.Fatalf("tokens %+v", got)
	}
	if got.CacheRead != 50 || got.CacheCreate != 5 {
		t.Fatalf("cache %+v", got)
	}
	if got.Vendor != "moonshot" || got.Source != "opencode" {
		t.Fatalf("axes %+v", got)
	}
	if got.SessionID != "s1" {
		t.Fatalf("session=%q", got.SessionID)
	}
}

func TestPartTokensNotDoubleCounted(t *testing.T) {
	dir := t.TempDir()
	dbPath := filepath.Join(dir, "opencode.db")
	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE message (id TEXT, session_id TEXT, time_created INTEGER, time_updated INTEGER, data TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE part (id TEXT, message_id TEXT, data TEXT)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO message (id, session_id, time_created, time_updated, data) VALUES (?, ?, ?, ?, ?)`,
		"a1", "s1", 2, 2, `{"role":"assistant","tokens":{"input":100,"output":10,"reasoning":2,"cache":{"read":50,"write":5}},"modelID":"k3","providerID":"kimi-for-coding"}`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`INSERT INTO part (id, message_id, data) VALUES (?, ?, ?)`,
		"p1", "a1", `{"type":"step-finish","tokens":{"input":999,"output":999,"reasoning":999,"cache":{"read":999,"write":999}}}`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}

	var evs []event.UsageEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "opencode", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 100 || evs[0].Output != 12 {
		t.Fatalf("double-counted %+v", evs[0])
	}
}

func TestSQLAvoidsSecretsTables(t *testing.T) {
	src, err := os.ReadFile("opencode.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(src))
	for _, banned := range []string{"account", "credential"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("production SQL must not mention %q", banned)
		}
	}
}
