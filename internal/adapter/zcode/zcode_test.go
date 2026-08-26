package zcode

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

const secret = "sk-zcode-fixture-SECRETVALUE"

func writeDB(t *testing.T, dir string, ddl string) string {
	t.Helper()
	dbDir := filepath.Join(dir, ".zcode", "cli", "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dbDir, "db.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(ddl); err != nil {
		t.Fatal(err)
	}
	return path
}

const modernDDL = `
CREATE TABLE model_usage (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  turn_id TEXT,
  model_id TEXT,
  started_at INTEGER,
  completed_at INTEGER,
  duration_ms INTEGER,
  input_tokens INTEGER,
  output_tokens INTEGER,
  reasoning_tokens INTEGER,
  cache_read_input_tokens INTEGER,
  cache_creation_input_tokens,
  computed_total_tokens INTEGER,
  agent TEXT,
  mode TEXT
);
CREATE TABLE session (id TEXT PRIMARY KEY, directory TEXT, path TEXT);
`

func collect(t *testing.T, home string) ([]event.UsageEvent, []event.TurnEvent) {
	t.Helper()
	roots := (Adapter{}).Discover(testhome.New(home))
	if len(roots) != 1 {
		t.Fatalf("roots=%v", roots)
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := (Adapter{}).Parse(roots[0], func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	return evs, turns
}

func TestModernInclusiveRows(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, modernDDL+`
INSERT INTO session (id, directory, path) VALUES ('s1', '/work/proj', NULL);
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at, completed_at, duration_ms,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens, agent, mode)
VALUES
  -- inclusive: total == input+output, so input absorbs cache, output absorbs reasoning
  ('r1', 's1', 't1', 'glm-5.2', 1786267140000, 1786267148269, 8269,
   100, 50, 10, 20, 5, 150, 'zcode', 'agent'),
  -- exclusive: total == input+output+cache+reasoning, pass through
  ('r2', 's1', 't1', 'glm-5.2', 1786267150000, 1786267151000, 1000,
   75, 40, 10, 20, 5, 150, 'zcode', 'agent'),
  -- NULL total on modern schema: shape unknown, pass through
  ('r3', 's1', 't2', 'deepseek-v4-flash', 1786267160000, NULL, NULL,
   7, 3, 0, 1, 0, NULL, 'zcode', 'agent'),
  -- all-zero row must not count
  ('r4', 's1', 't2', 'glm-5.2', 1786267170000, NULL, NULL,
   0, 0, 0, 0, 0, 0, 'zcode', 'agent');
`)
	evs, turns := collect(t, dir)
	if len(evs) != 3 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 75 || evs[0].CacheRead != 20 || evs[0].CacheCreate != 5 || evs[0].Output != 40 || evs[0].Reasoning != 10 {
		t.Fatalf("inclusive row must subtract overlaps: %+v", evs[0])
	}
	if evs[1].Miss != 75 || evs[1].CacheRead != 20 || evs[1].CacheCreate != 5 || evs[1].Output != 40 {
		t.Fatalf("exclusive row passes through: %+v", evs[1])
	}
	if evs[2].Miss != 7 || evs[2].CacheRead != 1 || evs[2].Output != 3 {
		t.Fatalf("null-total row passes through: %+v", evs[2])
	}
	if evs[0].Vendor != "zhipu" || evs[0].Workspace != "/work/proj" || evs[0].SessionID != "s1" {
		t.Fatalf("axes %+v", evs[0])
	}
	if evs[2].Vendor != "deepseek" {
		t.Fatalf("non-GLM model must keep its own vendor: %+v", evs[2])
	}
	if len(turns) != 2 {
		t.Fatalf("one turn per distinct turn_id: %+v", turns)
	}
}

func TestLegacySchemaSubtractsUnconditionally(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, `
CREATE TABLE model_usage (
  id TEXT PRIMARY KEY,
  session_id TEXT,
  turn_id TEXT,
  model_id TEXT,
  started_at INTEGER,
  completed_at INTEGER,
  duration_ms INTEGER,
  input_tokens INTEGER,
  output_tokens INTEGER,
  reasoning_tokens INTEGER,
  cache_read_input_tokens INTEGER,
  cache_creation_input_tokens INTEGER,
  agent TEXT,
  mode TEXT
);
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens)
VALUES ('r1', 's1', 't1', 'GLM-5.2', 1786267140000, 100, 50, 10, 20, 5);
`)
	evs, turns := collect(t, dir)
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 75 || evs[0].Output != 40 || evs[0].CacheRead != 20 {
		t.Fatalf("legacy rows are always inclusive: %+v", evs[0])
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
}

func TestNoSecretLeaksFromOtherTables(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, modernDDL+`
INSERT INTO session (id, directory, path) VALUES ('s1', '/work/proj', '`+secret+`');
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES ('r1', 's1', 't1', 'glm-5.2', 1786267140000, 10, 5, 0, 0, 0, 15);
`)
	evs, _ := collect(t, dir)
	if len(evs) != 1 {
		t.Fatalf("events=%d", len(evs))
	}
	if strings.Contains(evs[0].Workspace, secret) || strings.Contains(evs[0].SessionID, secret) {
		t.Fatalf("secret leaked onto event: %+v", evs[0])
	}
}

func TestEnvOverridePointsAtDBFile(t *testing.T) {
	dir := t.TempDir()
	dbPath := writeDB(t, dir, modernDDL+`
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES ('r1', 's1', 't1', 'glm-5.2', 1786267140000, 4, 2, 0, 0, 0, 6);
`)
	t.Setenv("ZCODE_DB", dbPath)
	roots := (Adapter{}).Discover(testhome.New(t.TempDir()))
	if len(roots) != 1 || roots[0].Path != dbPath {
		t.Fatalf("roots=%v", roots)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(roots[0], func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 4 {
		t.Fatalf("%+v", evs)
	}
}

func TestMissingDBIsSilent(t *testing.T) {
	dir := t.TempDir()
	if roots := (Adapter{}).Discover(testhome.New(dir)); len(roots) != 0 {
		t.Fatalf("no db, no roots: %v", roots)
	}
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "zcode", Path: dir}, func(event.UsageEvent) {
		t.Fatal("no events without a db")
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
}

func TestGarbageDBDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, ".zcode", "cli", "db")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dbDir, "db.sqlite"), []byte("not sqlite at all"), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "zcode", Path: filepath.Join(dir, ".zcode")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {})
	if err == nil {
		t.Fatal("a corrupt db should surface an error")
	}
	if len(evs) != 0 {
		t.Fatalf("corrupt db must not emit: %+v", evs)
	}
	if strings.Contains(err.Error(), dir) {
		t.Fatalf("error must not leak paths: %v", err)
	}
}

func TestSessionWithoutWorkspaceColumnsKeepsUsage(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, `
CREATE TABLE model_usage (
  id TEXT PRIMARY KEY, session_id TEXT, turn_id TEXT, model_id TEXT,
  started_at INTEGER, completed_at INTEGER, duration_ms INTEGER,
  input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
  cache_read_input_tokens INTEGER, cache_creation_input_tokens INTEGER,
  computed_total_tokens INTEGER, agent TEXT, mode TEXT
);
CREATE TABLE session (id TEXT PRIMARY KEY);
INSERT INTO session (id) VALUES ('s1');
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES ('r1', 's1', 't1', 'glm-5.2', 1786267140000, 10, 5, 0, 0, 0, 15);
`)
	evs, _ := collect(t, dir)
	if len(evs) != 1 {
		t.Fatalf("usage must survive a column-poor session table: %+v", evs)
	}
	if evs[0].Workspace != "" {
		t.Fatalf("workspace degrades to unlabeled, not error: %+v", evs[0])
	}
	if evs[0].Miss != 10 || evs[0].Output != 5 {
		t.Fatalf("usage numbers intact: %+v", evs[0])
	}
}

func TestSessionDirectoryOnlyColumnKeepsWorkspace(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, `
CREATE TABLE model_usage (
  id TEXT PRIMARY KEY, session_id TEXT, turn_id TEXT, model_id TEXT,
  started_at INTEGER, completed_at INTEGER, duration_ms INTEGER,
  input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
  cache_read_input_tokens INTEGER, cache_creation_input_tokens INTEGER,
  computed_total_tokens INTEGER, agent TEXT, mode TEXT
);
CREATE TABLE session (id TEXT PRIMARY KEY, directory TEXT);
INSERT INTO session (id, directory) VALUES ('s1', '/work/proj');
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES ('r1', 's1', 't1', 'glm-5.2', 1786267140000, 10, 5, 0, 0, 0, 15);
`)
	evs, _ := collect(t, dir)
	if len(evs) != 1 || evs[0].Workspace != "/work/proj" {
		t.Fatalf("directory-only session schema: %+v", evs)
	}
}

func TestNullIDRowsStayDistinct(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, modernDDL+`
INSERT INTO model_usage
  (id, session_id, turn_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES
  (NULL, 's1', 't1', 'glm-5.2', 1786267140000, 10, 5, 0, 0, 0, 15),
  (NULL, 's1', 't1', 'glm-5.2', 1786267150000, 20, 6, 0, 0, 0, 26);
`)
	evs, _ := collect(t, dir)
	if len(evs) != 2 {
		t.Fatalf("NULL id rows must not fail the source: %+v", evs)
	}
	if evs[0].RequestID == evs[1].RequestID || evs[0].RequestID == "zcode:" {
		t.Fatalf("NULL id rows need distinct RequestIDs: %+v", evs)
	}
	if evs[0].Miss != 10 || evs[1].Miss != 20 {
		t.Fatalf("usage numbers intact: %+v", evs)
	}
}

func TestMissingTurnIDColumnKeepsUsage(t *testing.T) {
	dir := t.TempDir()
	writeDB(t, dir, `
CREATE TABLE model_usage (
  id TEXT PRIMARY KEY, session_id TEXT, model_id TEXT,
  started_at INTEGER, completed_at INTEGER, duration_ms INTEGER,
  input_tokens INTEGER, output_tokens INTEGER, reasoning_tokens INTEGER,
  cache_read_input_tokens INTEGER, cache_creation_input_tokens INTEGER,
  computed_total_tokens INTEGER, agent TEXT, mode TEXT
);
INSERT INTO model_usage
  (id, session_id, model_id, started_at,
   input_tokens, output_tokens, reasoning_tokens,
   cache_read_input_tokens, cache_creation_input_tokens, computed_total_tokens)
VALUES ('r1', 's1', 'glm-5.2', 1786267140000, 10, 5, 0, 0, 0, 15);
`)
	evs, turns := collect(t, dir)
	if len(evs) != 1 {
		t.Fatalf("usage must survive without turn_id: %+v", evs)
	}
	if len(turns) != 0 {
		t.Fatalf("turns unavailable, not fabricated: %+v", turns)
	}
}
