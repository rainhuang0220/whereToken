package cursor

import (
	"database/sql"
	"errors"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

// The sqlite read is cached via index.LoadOrReplay: an unchanged state.vscdb
// replays cached events instead of re-parsing every blob.
func TestParseReplaysUnchangedDB(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`},
	}, nil)
	bindIndex(t, dir)

	miss1, out1, reqs1 := parseTotals(t, db)
	if miss1 != 100 || out1 != 10 || reqs1 != 1 {
		t.Fatalf("first parse miss=%d out=%d reqs=%d", miss1, out1, reqs1)
	}
	if !cursorDelta(index.ModeFull) {
		t.Fatalf("first scan must be a full parse: %+v", index.Deltas())
	}

	miss2, out2, reqs2 := parseTotals(t, db)
	if miss2 != miss1 || out2 != out1 || reqs2 != reqs1 {
		t.Fatalf("replay changed totals: %d/%d/%d -> %d/%d/%d", miss1, out1, reqs1, miss2, out2, reqs2)
	}
	if !cursorDelta(index.ModeUnchanged) {
		t.Fatalf("unchanged db must replay from cache: %+v", index.Deltas())
	}
}

// Append-only growth (a new bubble row) must re-parse fully and keep every
// old event: totals are monotonic, nothing is dropped.
func TestParseAppendKeepsTotalsMonotonic(t *testing.T) {
	dir := t.TempDir()
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:u1", value: `{"type":1,"createdAt":"2026-02-09T14:44:05.860Z","tokenCount":{"inputTokens":0,"outputTokens":0}}`},
		{key: "bubbleId:sess-a:a1", value: `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`},
	}, nil)
	bindIndex(t, dir)

	miss1, out1, _ := parseTotals(t, db)

	sq, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sq.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES ('bubbleId:sess-a:a2',
		'{"type":2,"createdAt":"2026-02-09T14:44:09.000Z","tokenCount":{"inputTokens":42,"outputTokens":8}}')`); err != nil {
		t.Fatal(err)
	}
	if err := sq.Close(); err != nil {
		t.Fatal(err)
	}
	bumpMtime(t, db, 2*time.Second)

	miss2, out2, reqs2 := parseTotals(t, db)
	if miss2 != miss1+42 || out2 != out1+8 || reqs2 != 2 {
		t.Fatalf("after append miss=%d out=%d reqs=%d (was %d/%d/1)", miss2, out2, reqs2, miss1, out1)
	}
	if cursorDelta(index.ModeIncremental) {
		t.Fatalf("sqlite growth must re-parse fully, not resume: %+v", index.Deltas())
	}
	if !cursorDelta(index.ModeFull) {
		t.Fatalf("append must be a full re-parse: %+v", index.Deltas())
	}

	// The old event is still there with its tokens intact.
	var found bool
	for _, e := range parseEvents(t, db) {
		if e.RequestID == "sess-a:a1" && e.Miss == miss1 && e.Output == out1 {
			found = true
		}
	}
	if !found {
		t.Fatal("append dropped the previously scanned event")
	}
}

// A failed re-parse must not overwrite the cache: once the file is back to
// the cached identity, the original events replay.
func TestParseMalformedBlobKeepsCachedEvents(t *testing.T) {
	dir := t.TempDir()
	good := `{"type":2,"createdAt":"2026-02-09T14:44:08.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`
	db := writeVscdb(t, dir, []kv{
		{key: "composerData:sess-a", value: `{"composerId":"sess-a","createdAt":1700000000000,"modelConfig":{"modelName":"claude-opus-4-6"},"usageData":{}}`},
		{key: "bubbleId:sess-a:a1", value: good},
	}, nil)
	bindIndex(t, dir)

	miss1, out1, _ := parseTotals(t, db)

	fi, err := os.Stat(db)
	if err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(db)
	if err != nil {
		t.Fatal(err)
	}

	sq, err := sql.Open("sqlite", db)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := sq.Exec(`UPDATE cursorDiskKV SET value = '{not json' WHERE key = 'bubbleId:sess-a:a1'`); err != nil {
		t.Fatal(err)
	}
	if err := sq.Close(); err != nil {
		t.Fatal(err)
	}
	bumpMtime(t, db, 2*time.Second)

	index.ResetDeltas()
	err = (Adapter{Offline: true}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(event.UsageEvent) {}, func(event.TurnEvent) {})
	if err == nil || errors.Is(err, errNoLocalAuth) {
		t.Fatalf("malformed blob must surface a parse error, got %v", err)
	}
	if cursorDelta(index.ModeUnchanged) {
		t.Fatalf("changed file must not replay: %+v", index.Deltas())
	}

	// Restore the exact cached identity: failed re-parse must have kept the
	// original cache entry, so this replays it.
	if err := os.WriteFile(db, saved, 0o644); err != nil {
		t.Fatal(err)
	}
	mt := fi.ModTime()
	if err := os.Chtimes(db, mt, mt); err != nil {
		t.Fatal(err)
	}
	miss3, out3, reqs3 := parseTotals(t, db)
	if miss3 != miss1 || out3 != out1 || reqs3 != 1 {
		t.Fatalf("cache lost events across failed re-parse: miss=%d out=%d reqs=%d", miss3, out3, reqs3)
	}
	if !cursorDelta(index.ModeUnchanged) {
		t.Fatalf("restored identity must replay the cache: %+v", index.Deltas())
	}
}

func bindIndex(t *testing.T, dir string) {
	t.Helper()
	st, err := index.Open(filepath.Join(dir, "index.v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	undo := index.Use(st)
	t.Cleanup(func() {
		undo()
		st.Close()
	})
}

// parseTotals parses offline (no API, no auth rows → errNoLocalAuth is
// expected) and sums local usage events.
func parseTotals(t *testing.T, db string) (miss, out int64, reqs int) {
	t.Helper()
	index.ResetDeltas()
	var n int64
	err := (Adapter{Offline: true}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		miss += e.Miss
		out += e.Output
		n++
	}, func(event.TurnEvent) {})
	if err != nil && !errors.Is(err, errNoLocalAuth) {
		t.Fatal(err)
	}
	return miss, out, int(n)
}

func parseEvents(t *testing.T, db string) []event.UsageEvent {
	t.Helper()
	var evs []event.UsageEvent
	err := (Adapter{Offline: true}).Parse(adapter.SourceRoot{ID: "cursor", Path: db}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {})
	if err != nil && !errors.Is(err, errNoLocalAuth) {
		t.Fatal(err)
	}
	return evs
}

func cursorDelta(mode string) bool {
	for _, d := range index.Deltas() {
		if d.Source == "cursor" && d.Mode == mode {
			return true
		}
	}
	return false
}

func bumpMtime(t *testing.T, path string, d time.Duration) {
	t.Helper()
	later := time.Now().Add(d)
	if err := os.Chtimes(path, later, later); err != nil {
		t.Fatal(err)
	}
}
