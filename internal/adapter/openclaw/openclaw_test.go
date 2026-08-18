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
	if len(evs) != 1 || evs[0].Miss != 100 || evs[0].RequestID != "resp-1" {
		t.Fatalf("%+v", evs)
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

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
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
