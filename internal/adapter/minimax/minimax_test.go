package minimax

import (
	"database/sql"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	_ "modernc.org/sqlite"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

const secret = "sk-leak-fixture-SECRETVALUE99"

func writeLedger(t *testing.T, dir string, extraSQL string) string {
	t.Helper()
	dbDir := filepath.Join(dir, ".minimax", "v2", "sqlite")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dbDir, "runtime-state.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	stmts := `
CREATE TABLE local_runtime_token_usage (
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
  cache_write_tokens INTEGER NOT NULL,
  cost_usd REAL,
  raw TEXT
);
CREATE TABLE local_runtime_message_rows (
  id INTEGER PRIMARY KEY,
  session_id TEXT NOT NULL,
  msg_id TEXT NOT NULL,
  role TEXT,
  turn_id TEXT,
  created_at_ms INTEGER NOT NULL,
  data_json TEXT NOT NULL
);
CREATE TABLE local_runtime_sessions (
  session_id TEXT PRIMARY KEY,
  record_json TEXT NOT NULL,
  updated_at_ms INTEGER NOT NULL
);
INSERT INTO local_runtime_token_usage
  (id, session_id, agent_name, framework_type, turn_id, model, ts,
   input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens, cost_usd, raw)
VALUES
  (1, 's1', 'mavis', 'pi-agent', 'turn-a', 'minimax/MiniMax-M3', 1786267148269,
   100, 10, 0, 50, 5, 999999, '{"prompt":"do not emit"}'),
  (2, 's1', 'mavis', 'pi-agent', 'turn-a', 'minimax/MiniMax-M3', 1786267170642,
   20, 8, 2, 80, 0, 0, ''),
  (3, 's2', 'coder', 'pi-agent', 'turn-b', 'custom_provider:deepseek/deepseek-v4-flash', 1786267200000,
   7, 3, 0, 1, 0, 0, '');
INSERT INTO local_runtime_message_rows
  (session_id, msg_id, role, turn_id, created_at_ms, data_json)
VALUES
  ('s1', 'u1', 'user', 'turn-a', 1786267140000, '{"text":"prompt body"}'),
  ('s1', 'a1', 'assistant', 'turn-a', 1786267148000, '{"text":"reply"}'),
  ('s2', 'u2', 'user', 'turn-b', 1786267190000, '{"text":"other"}');
INSERT INTO local_runtime_sessions (session_id, record_json, updated_at_ms)
VALUES ('s1', '{"workspaceDir":"/tmp/demo","title":"secret-title"}', 1);
`
	if extraSQL != "" {
		stmts += extraSQL
	}
	if _, err := db.Exec(stmts); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	auth := filepath.Join(dir, ".minimax", "local-runtime.auth.json")
	if err := os.WriteFile(auth, []byte(`{"token":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	return path
}

func TestDiscoverAndParseLedger(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, "")
	a := Adapter{}
	roots := a.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "minimax" {
		t.Fatalf("roots=%+v", roots)
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := a.Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(t event.TurnEvent) { turns = append(turns, t) }); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 3 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Miss != 100 || evs[0].Output != 10 || evs[0].CacheRead != 50 || evs[0].CacheCreate != 5 {
		t.Fatalf("row1 %+v", evs[0])
	}
	if evs[0].Vendor != "minimax" || evs[0].Source != "minimax" {
		t.Fatalf("axes %+v", evs[0])
	}
	if evs[0].RequestID != "1" || evs[0].SessionID != "s1" {
		t.Fatalf("ids %+v", evs[0])
	}
	if evs[0].Workspace != "/tmp/demo" {
		t.Fatalf("workspace=%q", evs[0].Workspace)
	}
	if evs[0].Timestamp.IsZero() {
		t.Fatal("timestamp")
	}
	if evs[1].Reasoning != 2 || evs[1].Output != 8 {
		t.Fatalf("reasoning must not fold into output %+v", evs[1])
	}
	if evs[2].Vendor != "deepseek" {
		t.Fatalf("deepseek vendor %+v", evs[2])
	}
	if len(turns) != 2 {
		t.Fatalf("turns=%d", len(turns))
	}
	for _, e := range evs {
		blob := e.RequestID + e.Model + e.SessionID + e.Workspace
		if strings.Contains(blob, "prompt") || strings.Contains(blob, "secret-title") || strings.Contains(blob, secret) {
			t.Fatalf("leaked %+v", e)
		}
		if e.Quality != event.QualityAuthoritative || e.Derivation != event.DeriveRaw {
			t.Fatalf("meta %+v", e)
		}
	}
}

func TestSameTurnRowsStayDistinctRequests(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, "")
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	ids := map[string]int{}
	var miss int64
	for _, e := range evs {
		ids[e.RequestID]++
		if e.SessionID == "s1" {
			miss += e.Miss
		}
	}
	if len(ids) != 3 {
		t.Fatalf("request ids=%v", ids)
	}
	if miss != 120 {
		t.Fatalf("same-turn rows must be summed later, not max-merged here: miss=%d", miss)
	}
}

func TestSameTurnRowsAggregateBySumNotMax(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, "")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(tr event.TurnEvent) { turns = append(turns, tr) }); err != nil {
		t.Fatal(err)
	}
	sum := metric.Aggregate(evs, turns)
	if sum.All.Requests != 3 {
		t.Fatalf("requests=%d want 3 distinct rows", sum.All.Requests)
	}
	if sum.All.Miss != 100+20+7 || sum.All.Output != 10+8+3 {
		t.Fatalf("same-turn rows must add, not max-merge: %+v", sum.All)
	}
	if sum.All.Total() != (100+50+5+10)+(20+80+8)+(7+1+3) {
		t.Fatalf("total=%d", sum.All.Total())
	}
	if sum.All.UserTurns != 2 {
		t.Fatalf("turns=%d", sum.All.UserTurns)
	}
}

func TestTimestampsSurviveSinceWindow(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, "")
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	loc := time.UTC
	inside := metric.Window{
		From: time.Date(2026, 8, 9, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 8, 10, 0, 0, 0, 0, loc),
	}
	before := metric.Window{
		From: time.Date(2026, 8, 1, 0, 0, 0, 0, loc),
		To:   time.Date(2026, 8, 8, 0, 0, 0, 0, loc),
	}
	var keep, early int
	for _, e := range evs {
		if e.Timestamp.IsZero() {
			t.Fatalf("usage rows must be dated: %+v", e)
		}
		if inside.Contains(e.Timestamp, loc) {
			keep++
		}
		if before.Contains(e.Timestamp, loc) {
			early++
		}
	}
	if keep != 3 || early != 0 {
		t.Fatalf("keep=%d early=%d", keep, early)
	}
}

func TestNegativeTokensClampedAndZeroSkipped(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, `
INSERT INTO local_runtime_token_usage
  (id, session_id, agent_name, framework_type, turn_id, model, ts,
   input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens)
VALUES
  (10, 's1', 'mavis', 'pi-agent', 't', 'minimax/MiniMax-M3', 1, -4, 1, 0, 0, 0),
  (11, 's1', 'mavis', 'pi-agent', 't', 'minimax/MiniMax-M3', 1, 0, 0, 0, 0, 0);
`)
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	var got *event.UsageEvent
	for i := range evs {
		if evs[i].RequestID == "10" {
			got = &evs[i]
		}
		if evs[i].RequestID == "11" {
			t.Fatal("all-zero row must be skipped")
		}
	}
	if got == nil || got.Miss != 0 || got.Output != 1 {
		t.Fatalf("clamp %+v", got)
	}
}

func TestNullModelDoesNotAbortLaterRows(t *testing.T) {
	dir := t.TempDir()
	writeLedger(t, dir, `
INSERT INTO local_runtime_token_usage
  (id, session_id, agent_name, framework_type, turn_id, model, ts,
   input_tokens, output_tokens, reasoning_tokens, cache_read_tokens, cache_write_tokens)
VALUES
  (20, 's1', 'mavis', 'pi-agent', 't', NULL, 1786267148269, 9, 1, 0, 0, 0),
  (21, 's1', 'mavis', 'pi-agent', 't', 'minimax/MiniMax-M3', 1786267148269, 3, 1, 0, 0, 0);
`)
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	var nullRow, later *event.UsageEvent
	for i := range evs {
		switch evs[i].RequestID {
		case "20":
			nullRow = &evs[i]
		case "21":
			later = &evs[i]
		}
	}
	if nullRow == nil || nullRow.Miss != 9 {
		t.Fatalf("null model row must still emit tokens: %+v", evs)
	}
	if later == nil || later.Miss != 3 {
		t.Fatalf("later row dropped after null model: %+v", evs)
	}
}

func TestMissingUsageTableIsEmpty(t *testing.T) {
	dir := t.TempDir()
	dbDir := filepath.Join(dir, ".minimax", "v2", "sqlite")
	if err := os.MkdirAll(dbDir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dbDir, "runtime-state.sqlite")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE local_runtime_agents (name TEXT)`); err != nil {
		t.Fatal(err)
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	var n int
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "minimax", Path: filepath.Join(dir, ".minimax")}, func(event.UsageEvent) {
		n++
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("n=%d", n)
	}
}

func TestSQLAvoidsBodiesAndBilledCost(t *testing.T) {
	src, err := os.ReadFile("minimax.go")
	if err != nil {
		t.Fatal(err)
	}
	lower := strings.ToLower(string(src))
	for _, banned := range []string{"cost_usd", "data_json", "credential"} {
		if strings.Contains(lower, banned) {
			t.Fatalf("production SQL must not mention %q", banned)
		}
	}
}

func TestDiscoverEmptyHome(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}
