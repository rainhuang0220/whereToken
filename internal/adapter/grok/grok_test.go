package grok

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func fixtureRoot(t *testing.T) string {
	t.Helper()
	_, file, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "grok", "sessions")
}

func TestDiscoverGrokSessions(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sess := filepath.Join(dir, ".grok", "sessions")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "grok" || roots[0].Path != sess {
		t.Fatalf("roots=%+v", roots)
	}
}

func TestDiscoverMissingGrokIsSilent(t *testing.T) {
	t.Parallel()
	roots := Adapter{}.Discover(testhome.New(t.TempDir()))
	if len(roots) != 0 {
		t.Fatalf("roots=%+v", roots)
	}
}

func TestParseUpdatesJSONL(t *testing.T) {
	t.Parallel()
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	root := fixtureRoot(t)
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: root}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(te event.TurnEvent) {
		turns = append(turns, te)
	}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if len(turns) != 2 {
		t.Fatalf("turns=%d", len(turns))
	}
	sum := metric.Aggregate(evs, turns)
	if sum.All.Requests != 2 || sum.All.UserTurns != 2 {
		t.Fatalf("requests=%d turns=%d", sum.All.Requests, sum.All.UserTurns)
	}
	if sum.All.Miss != 40 || sum.All.CacheRead != 110 || sum.All.CacheCreate != 20 || sum.All.Output != 15 {
		t.Fatalf("totals %+v", sum.All)
	}
	if sum.All.Total() != 185 {
		t.Fatalf("total=%d", sum.All.Total())
	}
	byID := map[string]event.UsageEvent{}
	for _, e := range evs {
		byID[e.RequestID] = e
	}
	p1 := byID["p1"]
	if p1.Source != "grok" || p1.Vendor != "xai" || p1.Model != "grok-4.6-build" {
		t.Fatalf("p1 axes %+v", p1)
	}
	if p1.Quality != event.QualityAuthoritative {
		t.Fatalf("quality=%s", p1.Quality)
	}
	if p1.Miss != 20 || p1.CacheRead != 80 || p1.CacheCreate != 20 || p1.Output != 10 || p1.Reasoning != 4 {
		t.Fatalf("p1 tokens %+v", p1)
	}
	if p1.Workspace != "/tmp/demo" || p1.SessionID != "s1" {
		t.Fatalf("p1 context %+v", p1)
	}
	if !p1.Timestamp.Equal(time.UnixMilli(1700000001500).UTC()) {
		t.Fatalf("p1 ts=%s", p1.Timestamp)
	}
	p2 := byID["p2"]
	if p2.Model != "grok" || p2.Vendor != "xai" {
		t.Fatalf("p2 axes %+v", p2)
	}
	if p2.Miss != 20 || p2.CacheRead != 30 {
		t.Fatalf("p2 tokens %+v", p2)
	}
}

func TestParseDoesNotReadAuthJSON(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sess := filepath.Join(dir, "ws", "sid")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "auth.json"), []byte(`{"prompt_id":"secret","usage":{"inputTokens":999999,"outputTokens":9}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":1700000000,"params":{"update":{"sessionUpdate":"turn_completed","prompt_id":"p","usage":{"inputTokens":10,"outputTokens":1,"cachedReadTokens":0,"cacheCreationTokens":0}}}}
`
	if err := os.WriteFile(filepath.Join(sess, "updates.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].RequestID != "p" || evs[0].Miss != 10 {
		t.Fatalf("%+v", evs)
	}
}

func TestParseIgnoresAuthAndChatHistory(t *testing.T) {
	t.Parallel()
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: fixtureRoot(t)}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.RequestID == "secret" || e.Miss == 999999 || e.Miss == 888888 {
			t.Fatalf("read a non-ledger file: %+v", e)
		}
	}
}

func TestParseSkipsTurnWithoutPromptID(t *testing.T) {
	t.Parallel()
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: fixtureRoot(t)}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	for _, e := range evs {
		if e.Miss == 999 || e.RequestID == "e5" {
			t.Fatalf("prompt-less row became a request: %+v", e)
		}
	}
}

func TestMissClampsWhenCacheExceedsInput(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sess := filepath.Join(dir, "ws", "sid")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":1700000000,"params":{"sessionId":"sid","update":{"sessionUpdate":"turn_completed","prompt_id":"p","usage":{"inputTokens":10,"outputTokens":1,"cachedReadTokens":80,"cacheCreationTokens":5}},"_meta":{"agentTimestampMs":1700000000000}}}
`
	if err := os.WriteFile(filepath.Join(sess, "updates.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 0 || evs[0].CacheRead != 80 {
		t.Fatalf("%+v", evs)
	}
}

func TestMultiModelSplitsRequestIDs(t *testing.T) {
	t.Parallel()
	dir := t.TempDir()
	sess := filepath.Join(dir, "ws", "sid")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"timestamp":1700000000,"params":{"update":{"sessionUpdate":"turn_completed","prompt_id":"p","usage":{"inputTokens":30,"outputTokens":3,"cachedReadTokens":0,"cacheCreationTokens":0,"modelUsage":{"grok-4.6-build":{"inputTokens":10,"outputTokens":1},"other-grok":{"inputTokens":20,"outputTokens":2}}}}}}
`
	if err := os.WriteFile(filepath.Join(sess, "updates.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	sum := metric.Aggregate(evs, nil)
	if sum.All.Requests != 2 || sum.All.Miss != 30 || sum.All.Output != 3 {
		t.Fatalf("%+v", sum.All)
	}
	ids := map[string]bool{}
	for _, e := range evs {
		ids[e.RequestID] = true
	}
	if !ids["p:grok-4.6-build"] || !ids["p:other-grok"] {
		t.Fatalf("ids=%v", ids)
	}
}

func TestParseDoesNotKeepUSD(t *testing.T) {
	t.Parallel()
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "grok", Path: fixtureRoot(t)}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	raw, err := json.Marshal(evs)
	if err != nil {
		t.Fatal(err)
	}
	s := strings.ToLower(string(raw))
	if strings.Contains(s, "usd") || strings.Contains(s, "cost") || strings.Contains(s, "999999") {
		t.Fatalf("usd/cost leaked: %s", raw)
	}
}
