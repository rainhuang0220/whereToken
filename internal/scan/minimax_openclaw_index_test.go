package scan

import (
	"database/sql"
	"os"
	"path/filepath"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/minimax"
	"github.com/rainhuang0220/whereToken/internal/adapter/openclaw"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/index"
)

func TestScanMiniMaxSecondPassIsUnchanged(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	t.Setenv("WHERETOKEN_NO_INDEX", "")
	t.Setenv("WHERETOKEN_INDEX", "")
	dir := t.TempDir()
	writeMiniMaxLedger(t, dir, 1, 10)
	home := testhome.New(dir)
	ads := []adapter.Adapter{minimax.Adapter{}}
	first := Run(home, ads)
	if first.Summary.All.Miss != 10 || first.Summary.All.Requests != 1 {
		t.Fatalf("first %+v", first.Summary.All)
	}
	if !hasMode(first.Deltas, "minimax", index.ModeFull) {
		t.Fatalf("first deltas=%+v", first.Deltas)
	}
	second := Run(home, ads)
	if second.Summary.All.Miss != 10 || second.Summary.All.Requests != 1 {
		t.Fatalf("second %+v", second.Summary.All)
	}
	if !hasMode(second.Deltas, "minimax", index.ModeUnchanged) {
		t.Fatalf("sqlite replay must be unchanged: %+v", second.Deltas)
	}
}

func TestScanMiniMaxChangeIsFullNotIncremental(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	t.Setenv("WHERETOKEN_NO_INDEX", "")
	t.Setenv("WHERETOKEN_INDEX", "")
	dir := t.TempDir()
	writeMiniMaxLedger(t, dir, 1, 10)
	home := testhome.New(dir)
	ads := []adapter.Adapter{minimax.Adapter{}}
	_ = Run(home, ads)
	writeMiniMaxLedger(t, dir, 2, 7)
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(miniMaxDB(dir), later, later)
	second := Run(home, ads)
	if second.Summary.All.Miss != 10+7 || second.Summary.All.Requests != 2 {
		t.Fatalf("changed sqlite must fully reparse: %+v", second.Summary.All)
	}
	if hasMode(second.Deltas, "minimax", index.ModeIncremental) {
		t.Fatalf("sqlite has no byte resume: %+v", second.Deltas)
	}
	if !hasMode(second.Deltas, "minimax", index.ModeFull) {
		t.Fatalf("want full %+v", second.Deltas)
	}
}

func TestScanOpenClawAppendIsIncremental(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	t.Setenv("WHERETOKEN_NO_INDEX", "")
	t.Setenv("WHERETOKEN_INDEX", "")
	dir := t.TempDir()
	path := writeOpenClawSession(t, dir, lineOpenClaw("a", 4))
	home := testhome.New(dir)
	ads := []adapter.Adapter{openclaw.Adapter{}}
	first := Run(home, ads)
	if first.Summary.All.Miss != 4 || first.Summary.All.Requests != 1 {
		t.Fatalf("first %+v", first.Summary.All)
	}
	if !hasMode(first.Deltas, "openclaw", index.ModeFull) {
		t.Fatalf("first deltas=%+v", first.Deltas)
	}
	f, err := os.OpenFile(path, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := f.WriteString(lineOpenClaw("b", 8)); err != nil {
		t.Fatal(err)
	}
	f.Close()
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)
	second := Run(home, ads)
	if second.Summary.All.Miss != 12 || second.Summary.All.Requests != 2 {
		t.Fatalf("after append %+v", second.Summary.All)
	}
	if !hasMode(second.Deltas, "openclaw", index.ModeIncremental) {
		t.Fatalf("want incremental %+v", second.Deltas)
	}
}

func TestScanOpenClawTruncateIsFull(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	t.Setenv("WHERETOKEN_NO_INDEX", "")
	t.Setenv("WHERETOKEN_INDEX", "")
	dir := t.TempDir()
	path := writeOpenClawSession(t, dir, lineOpenClaw("old", 99)+lineOpenClaw("old2", 1))
	home := testhome.New(dir)
	ads := []adapter.Adapter{openclaw.Adapter{}}
	_ = Run(home, ads)
	if err := os.WriteFile(path, []byte(lineOpenClaw("new", 3)), 0o644); err != nil {
		t.Fatal(err)
	}
	second := Run(home, ads)
	if second.Summary.All.Miss != 3 || second.Summary.All.Requests != 1 {
		t.Fatalf("truncate must rescan: %+v", second.Summary.All)
	}
	if !hasMode(second.Deltas, "openclaw", index.ModeFull) {
		t.Fatalf("truncate deltas=%+v", second.Deltas)
	}
}

func miniMaxDB(dir string) string {
	return filepath.Join(dir, ".minimax", "v2", "sqlite", "runtime-state.sqlite")
}

func writeMiniMaxLedger(t *testing.T, dir string, id, miss int64) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(miniMaxDB(dir)), 0o755); err != nil {
		t.Fatal(err)
	}
	db, err := sql.Open("sqlite", miniMaxDB(dir))
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`
CREATE TABLE IF NOT EXISTS local_runtime_token_usage (
  id INTEGER PRIMARY KEY,
  session_id TEXT NOT NULL,
  agent_name TEXT NOT NULL,
  framework_type TEXT NOT NULL,
  turn_id TEXT,
  model TEXT,
  ts INTEGER NOT NULL,
  input_tokens INTEGER NOT NULL,
  output_tokens INTEGER NOT NULL,
  reasoning_tokens INTEGER NOT NULL,
  cache_read_tokens INTEGER NOT NULL,
  cache_write_tokens INTEGER NOT NULL
);
INSERT INTO local_runtime_token_usage
  (id, session_id, agent_name, framework_type, turn_id, model, ts,
   input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens)
VALUES (?, 's1', 'mavis', 'pi-agent', 't', 'minimax/MiniMax-M3', 1786267148269, ?, 1, 0, 0, 0);
`, id, miss); err != nil {
		t.Fatal(err)
	}
}

func lineOpenClaw(id string, miss int) string {
	return `{"type":"message","timestamp":"2026-07-26T11:03:33Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"` + id + `","usage":{"input":` + itoa(miss) + `,"output":1}}}` + "\n"
}

func writeOpenClawSession(t *testing.T, dir, body string) string {
	t.Helper()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(sess, "s.jsonl")
	if err := os.WriteFile(path, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	return path
}
