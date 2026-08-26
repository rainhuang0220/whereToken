package gemini

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

const secret = "sk-leak-fixture-SECRETVALUE99"

func TestDiscoverAndParseJSONL(t *testing.T) {
	dir := t.TempDir()
	chats := filepath.Join(dir, ".gemini", "tmp", "projhash", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"type":"user","id":"u1","timestamp":"2026-08-20T10:00:00.000Z"}`,
		`{"type":"gemini","id":"g1","timestamp":"2026-08-20T10:00:01.000Z","model":"gemini-2.5-pro","tokens":{"input":120,"output":10,"cached":20,"thoughts":5,"total":130}}`,
		`{not json`,
		`{"type":"gemini","id":"g2","timestamp":"2026-08-20T10:00:02.000Z","model":"gemini-2.5-pro","tokens":{"input":50,"output":4,"cached":0,"total":54}}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(chats, "session-abc.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gemini", "oauth_creds.json"), []byte(`{"token":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".gemini", "settings.json"), []byte(`{"GEMINI_API_KEY":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	a := Adapter{}
	roots := a.Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("roots=%+v", roots)
	}
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	if err := a.Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(tr event.TurnEvent) { turns = append(turns, tr) }); err != nil {
		t.Fatal(err)
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 100 || evs[0].CacheRead != 20 || evs[0].Output != 15 || evs[0].Reasoning != 5 {
		t.Fatalf("row1 %+v", evs[0])
	}
	if evs[0].Vendor != "google" || evs[0].RequestID != "g1" {
		t.Fatalf("ids %+v", evs[0])
	}
	if evs[0].Miss+evs[0].CacheRead+evs[0].Output != 135 {
		t.Fatalf("official total is input+output+thoughts; reasoning must not be added again: %+v", evs[0])
	}
	for _, e := range evs {
		blob := e.RequestID + e.Model + e.SessionID
		if strings.Contains(blob, secret) {
			t.Fatalf("leaked %+v", e)
		}
	}
}

func TestMalformedDoesNotDropLater(t *testing.T) {
	dir := t.TempDir()
	chats := filepath.Join(dir, ".gemini", "tmp", "p", "chats")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(chats, "session-x.jsonl"), []byte("{nope\n"+`{"type":"gemini","id":"ok","tokens":{"input":3,"output":1}}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "gemini", Path: filepath.Join(dir, ".gemini")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 3 {
		t.Fatalf("%+v", evs)
	}
}

func TestSubagentChatsAreRead(t *testing.T) {
	dir := t.TempDir()
	chats := filepath.Join(dir, ".gemini", "tmp", "p", "chats", "parent-id")
	if err := os.MkdirAll(chats, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"gemini","id":"sub1","tokens":{"input":8,"output":2,"cached":0}}` + "\n"
	if err := os.WriteFile(filepath.Join(chats, "child.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "gemini", Path: filepath.Join(dir, ".gemini")}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 8 {
		t.Fatalf("subagent %+v", evs)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}

func TestConfigDirWithoutChatsIsNotUsage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".gemini"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := (Adapter{}).Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("config dir should be discovered: %+v", roots)
	}
	if err := (Adapter{}).Parse(roots[0], func(event.UsageEvent) {
		t.Fatal("no events from empty gemini home")
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
}
