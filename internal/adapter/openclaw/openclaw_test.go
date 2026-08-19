package openclaw

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

const secret = "sk-leak-fixture-SECRETVALUE99"

func writeSession(t *testing.T, dir, name, body string) {
	t.Helper()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, name), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverAndParseUsage(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "abc.jsonl", strings.Join([]string{
		`{"type":"session","id":"sess-1","timestamp":"2026-07-26T11:03:31.353Z","cwd":"/tmp/demo"}`,
		`{"type":"message","id":"line-uuid-1","timestamp":"2026-07-26T11:03:32Z","message":{"role":"user","content":"PROMPT","timestamp":"2026-07-26T11:03:32Z"}}`,
		`{"type":"message","id":"line-uuid-2","timestamp":"2026-07-26T11:03:33Z","message":{"role":"assistant","provider":"minimax-portal","model":"MiniMax-M2.1","responseId":"resp-1","usage":{"input":100,"output":10,"cacheRead":50,"cacheWrite":5,"cost":{"usd":9},"totalTokens":999},"timestamp":"2026-07-26T11:03:33Z"}}`,
		`{"type":"message","id":"line-uuid-3","timestamp":"2026-07-26T11:03:34Z","message":{"role":"toolResult","content":"tool","usage":{"input":1,"output":1}}}`,
		`{not json`,
		`{"type":"message","id":"line-uuid-4","timestamp":"2026-07-26T11:03:35Z","message":{"role":"assistant","provider":"github-copilot","model":"grok-code-fast-1","usage":{"input":7,"output":3,"cacheRead":1,"cacheWrite":0},"timestamp":"2026-07-26T11:03:35Z"}}`,
	}, "\n")+"\n")
	writeSession(t, dir, "abc.trajectory.jsonl", `{"type":"model.completed","data":{"usage":{"input":99999,"output":99999},"finalPromptText":"`+secret+`"}}`+"\n")
	cred := filepath.Join(dir, ".openclaw", "credentials")
	if err := os.MkdirAll(cred, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(cred, "github-copilot.token.json"), []byte(`{"token":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".openclaw", "openclaw.json"), []byte(`{"token":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	a := Adapter{}
	roots := a.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "openclaw" {
		t.Fatalf("roots=%+v", roots)
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := a.Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(tr event.TurnEvent) { turns = append(turns, tr) }); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 100 || evs[0].Output != 10 || evs[0].CacheRead != 50 || evs[0].CacheCreate != 5 {
		t.Fatalf("row1 %+v", evs[0])
	}
	if evs[0].Vendor != "minimax" || evs[0].RequestID != "resp-1" || evs[0].SessionID != "sess-1" {
		t.Fatalf("row1 ids %+v", evs[0])
	}
	if evs[0].Workspace != "/tmp/demo" {
		t.Fatalf("workspace=%q", evs[0].Workspace)
	}
	if evs[0].RequestID == "line-uuid-2" {
		t.Fatal("must not use per-line uuid as RequestID")
	}
	if evs[1].Vendor != "xai" || evs[1].Miss != 7 {
		t.Fatalf("copilot grok %+v", evs[1])
	}
	if strings.Contains(evs[1].RequestID, "line-uuid") {
		t.Fatalf("fallback request id leaked line uuid %q", evs[1].RequestID)
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
	for _, e := range evs {
		blob := e.RequestID + e.Model + e.SessionID + e.Workspace + e.Provider
		if strings.Contains(blob, secret) || strings.Contains(blob, "PROMPT") {
			t.Fatalf("leaked %+v", e)
		}
		if e.Quality != event.QualityAuthoritative || e.Derivation != event.DeriveRaw {
			t.Fatalf("meta %+v", e)
		}
	}
}

func TestIncompleteTailIsNotConsumed(t *testing.T) {
	dir := t.TempDir()
	store, err := index.Open(filepath.Join(dir, "idx.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	defer index.Use(store)()

	complete := `{"type":"message","timestamp":"2026-07-26T11:03:33Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"a","usage":{"input":4,"output":1}}}` + "\n"
	tail := `{"type":"message","timestamp":"2026-07-26T11:03:34Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"b","usage":{"input":8,"output":2}}}`
	writeSession(t, dir, "t.jsonl", complete+tail)
	root := adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(root, func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].RequestID != "a" {
		t.Fatalf("unterminated tail must not be parsed: %+v", evs)
	}

	path := filepath.Join(dir, ".openclaw", "agents", "main", "sessions", "t.jsonl")
	if err := os.WriteFile(path, []byte(complete+tail+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	later := time.Now().Add(2 * time.Second)
	_ = os.Chtimes(path, later, later)

	evs = nil
	if err := (Adapter{}).Parse(root, func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("completed tail must appear on the next scan: %+v", evs)
	}
	got := map[string]int64{}
	for _, e := range evs {
		got[e.RequestID] = e.Miss
	}
	if got["a"] != 4 || got["b"] != 8 {
		t.Fatalf("ids=%v", got)
	}
}

func TestMalformedDoesNotDropLaterRows(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "s.jsonl", "{nope\n"+`{"type":"message","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"r","usage":{"input":2,"output":1}}}`+"\n")
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 2 {
		t.Fatalf("%+v", evs)
	}
}

func TestParseCheckedInFixture(t *testing.T) {
	root := adapter.SourceRoot{ID: "openclaw", Path: filepath.Join("..", "..", "..", "testdata", "adapters", "openclaw")}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(root, func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	for _, e := range evs {
		got[e.RequestID] = e.Miss
	}
	if got["resp-1"] != 100 || got["resp-reset"] != 40 || got["resp-deleted"] != 400 || got["run-fixture:1"] != 25 {
		t.Fatalf("fixture ids=%v evs=%+v", got, evs)
	}
}

func TestNumericTopLevelTimestampStillCounts(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "n2.jsonl", `{"type":"message","id":"line-uuid","timestamp":1785323044000,"message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"r2","usage":{"input":6,"output":2}}}`+"\n")
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 6 || evs[0].Timestamp.IsZero() {
		t.Fatalf("numeric top-level timestamp must not drop the row: %+v", evs)
	}
	if evs[0].RequestID == "line-uuid" {
		t.Fatal("must not use per-line uuid")
	}
}

func TestNumericMessageTimestampStillCounts(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "n.jsonl", `{"type":"message","timestamp":"2026-07-26T11:03:33Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"r","timestamp":1785323044000,"usage":{"input":4,"output":1}}}`+"\n")
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 4 || evs[0].Timestamp.IsZero() {
		t.Fatalf("%+v", evs)
	}
}

func TestParseLiveHomeLayout(t *testing.T) {
	if os.Getenv("WHERETOKEN_LIVE_OPENCLAW") == "" {
		t.Skip("set WHERETOKEN_LIVE_OPENCLAW=1 to parse $HOME/.openclaw")
	}
	home := os.Getenv("HOME")
	if home == "" {
		t.Skip("no HOME")
	}
	if _, err := os.Stat(filepath.Join(home, ".openclaw", "agents")); err != nil {
		t.Skip("no live openclaw agents")
	}
	roots := (Adapter{}).Discover(testhome.New(home))
	if len(roots) != 1 {
		t.Fatalf("roots=%+v", roots)
	}
	var n int
	if err := (Adapter{}).Parse(roots[0], func(event.UsageEvent) { n++ }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if n == 0 {
		t.Fatalf("live ledger at %s produced no events", roots[0].Path)
	}
}

func TestParseSkipsRuntimeSQLite(t *testing.T) {
	dir := t.TempDir()
	writeSession(t, dir, "keep.jsonl", `{"type":"message","timestamp":"2026-08-19T11:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"r1","usage":{"input":4,"output":1},"timestamp":"2026-08-19T11:00:00Z"}}`+"\n")
	agentDir := filepath.Join(dir, ".openclaw", "agents", "main", "agent")
	if err := os.MkdirAll(agentDir, 0o755); err != nil {
		t.Fatal(err)
	}
	// Fake ledger-shaped JSONL under the runtime DB dir. Must not be parsed.
	poison := `{"type":"message","timestamp":"2026-08-19T11:00:01Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"secret-row","usage":{"input":99999,"output":1},"timestamp":"2026-08-19T11:00:01Z"}}` + "\n"
	for _, name := range []string{"openclaw-agent.sqlite", "usage.jsonl"} {
		if err := os.WriteFile(filepath.Join(agentDir, name), []byte(poison), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	var evs []event.UsageEvent
	roots := (Adapter{}).Discover(testhome.New(dir))
	if err := (Adapter{}).Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 4 || evs[0].RequestID == "secret-row" {
		t.Fatalf("runtime sqlite/auth dir leaked into usage: %+v", evs)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}

func TestDiscoverHonorsStateDir(t *testing.T) {
	dir := t.TempDir()
	reloc := filepath.Join(dir, "state")
	if err := os.Mkdir(reloc, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("OPENCLAW_STATE_DIR", reloc)
	t.Setenv("OPENCLAW_HOME", "")
	roots := (Adapter{}).Discover(testhome.New(t.TempDir()))
	if len(roots) != 1 || roots[0].Path != reloc {
		t.Fatalf("OPENCLAW_STATE_DIR roots=%+v", roots)
	}
}

func TestDeletedZstIsNotParsedAsJSONL(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	zst := filepath.Join(sess, "s.jsonl.deleted.2026-08-20T00-00-00.zst")
	if err := os.WriteFile(zst, []byte{0x28, 0xb5, 0x2f, 0xfd, 0x00, 0x00}, 0o644); err != nil {
		t.Fatal(err)
	}
	keep := `{"type":"message","timestamp":"2026-08-19T11:00:01Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"live","usage":{"input":4,"output":1}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "s.jsonl"), []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].RequestID != "live" {
		t.Fatalf("zstd archive must not be parsed as JSONL: %+v", evs)
	}
}

func TestSourceAvoidsCredentialWalk(t *testing.T) {
	src, err := os.ReadFile("openclaw.go")
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(src), "device-auth") || strings.Contains(string(src), "openclaw.json") {
		t.Fatal("must not open config/login files")
	}
}

func TestCheckpointJSONLIsSkipped(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	poison := `{"type":"message","timestamp":"2026-08-19T11:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"ckpt","usage":{"input":999,"output":1}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "s.checkpoint.abc.jsonl"), []byte(poison), 0o644); err != nil {
		t.Fatal(err)
	}
	keep := `{"type":"message","timestamp":"2026-08-19T11:00:01Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"live","usage":{"input":4,"output":1}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "s.jsonl"), []byte(keep), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].RequestID != "live" {
		t.Fatalf("compaction checkpoint must not count: %+v", evs)
	}
}

func TestParseResetAndDeletedArchives(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	active := `{"type":"message","timestamp":"2026-08-19T11:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"live","usage":{"input":4,"output":1}}}` + "\n"
	reset := `{"type":"message","timestamp":"2026-07-29T11:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"old","usage":{"input":40,"output":2}}}` + "\n"
	deleted := `{"type":"message","timestamp":"2026-07-26T08:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"gone","usage":{"input":400,"output":3}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "live.jsonl"), []byte(active), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, "live.jsonl.reset.2026-07-29T14-16-46.399Z"), []byte(reset), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, "gone.jsonl.deleted.2026-07-26T08-12-20.568Z"), []byte(deleted), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	var miss int64
	for _, e := range evs {
		got[e.RequestID] = e.Miss
		miss += e.Miss
	}
	if got["live"] != 4 || got["old"] != 40 || got["gone"] != 400 {
		t.Fatalf("reset/deleted archives must still count: %+v", evs)
	}
	if miss != 444 {
		t.Fatalf("miss=%d want 444", miss)
	}
}

func TestResetRenameDoesNotDropTokens(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	live := filepath.Join(sess, "s.jsonl")
	body := `{"type":"message","timestamp":"2026-07-26T11:03:33Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"hist","usage":{"input":99,"output":1}}}` + "\n"
	if err := os.WriteFile(live, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	root := adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}
	sum := func() int64 {
		var n int64
		if err := (Adapter{}).Parse(root, func(e event.UsageEvent) { n += e.Miss }, func(event.TurnEvent) {}); err != nil {
			t.Fatal(err)
		}
		return n
	}
	before := sum()
	if before != 99 {
		t.Fatalf("before=%d", before)
	}
	archived := filepath.Join(sess, "s.jsonl.reset.2026-08-19T12-00-00.000Z")
	if err := os.Rename(live, archived); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(live, []byte(`{"type":"message","timestamp":"2026-08-19T12:00:01Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"new","usage":{"input":3,"output":1}}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	after := sum()
	if after < before {
		t.Fatalf("reset must keep history: before=%d after=%d", before, after)
	}
	if after != 102 {
		t.Fatalf("after=%d want 102 (99 archived + 3 new)", after)
	}
}

func TestTrajectoryUsageOnlyWhenNoTranscript(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "freshman", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	const prompt = "SECRET-PROMPT-TEXT-MUST-NOT-LEAK"
	trajOnly := `{"type":"model.completed","ts":"2026-07-27T03:00:00.000Z","sessionId":"only-traj","runId":"run-1","provider":"minimax-portal","modelId":"MiniMax-M2.1","data":{"usage":{"input":25,"output":5,"cacheRead":10,"cacheWrite":2},"finalPromptText":"` + prompt + `","assistantTexts":["` + prompt + `"]}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "only-traj.trajectory.jsonl"), []byte(trajOnly), 0o644); err != nil {
		t.Fatal(err)
	}
	bothJSONL := `{"type":"message","timestamp":"2026-07-27T04:00:00Z","message":{"role":"assistant","model":"MiniMax-M2.1","responseId":"both","usage":{"input":7,"output":1}}}` + "\n"
	bothTraj := `{"type":"model.completed","ts":"2026-07-27T04:00:00.000Z","sessionId":"both","runId":"run-2","provider":"minimax-portal","modelId":"MiniMax-M2.1","data":{"usage":{"input":700,"output":1},"finalPromptText":"` + prompt + `"}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "both.jsonl"), []byte(bothJSONL), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(sess, "both.trajectory.jsonl"), []byte(bothTraj), 0o644); err != nil {
		t.Fatal(err)
	}

	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	got := map[string]int64{}
	var miss int64
	for _, e := range evs {
		got[e.RequestID] = e.Miss
		miss += e.Miss
		blob := e.RequestID + e.Model + e.SessionID + e.Workspace + e.Provider
		if strings.Contains(blob, prompt) {
			t.Fatalf("trajectory prompt leaked onto event %+v", e)
		}
	}
	if got["both"] != 7 {
		t.Fatalf("sibling trajectory must not replace/add on top of jsonl: %+v", evs)
	}
	if miss != 32 { // 25 from traj-only + 7 from jsonl
		t.Fatalf("traj-only session must count usage once: miss=%d evs=%+v", miss, evs)
	}
}

func TestTrajectoryMalformedDoesNotDropLaterRows(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, ".openclaw", "agents", "main", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	body := "{nope\n" + `{"type":"model.completed","ts":"2026-07-27T03:00:00.000Z","sessionId":"t","runId":"r","provider":"minimax-portal","modelId":"MiniMax-M2.1","data":{"usage":{"input":11,"output":2}}}` + "\n"
	if err := os.WriteFile(filepath.Join(sess, "t.trajectory.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "openclaw", Path: filepath.Join(dir, ".openclaw")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 11 {
		t.Fatalf("%+v", evs)
	}
}
