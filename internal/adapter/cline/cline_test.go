package cline

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
const extID = "saoudrizwan.claude-dev"

func TestDiscoverAndParseMetrics(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "Library", "Application Support", "Code", "User", "globalStorage", extID, "tasks", "task-1")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	msgs := `[
	  {"type":"say","say":"api_req_started","ts":1787200000000,"text":"{\"tokensIn\":10,\"tokensOut\":4,\"cacheReads\":2,\"cacheWrites\":1,\"model\":\"claude-sonnet-4.6\"}"},
	  {"type":"say","say":"text","ts":1787200001000,"text":"PROMPT ` + secret + `"},
	  {"type":"say","say":"subagent_usage","ts":1787200002000,"text":"{\"tokensIn\":5,\"tokensOut\":1}"}
	]`
	if err := os.WriteFile(filepath.Join(task, "ui_messages.json"), []byte(msgs), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(filepath.Dir(filepath.Dir(task)), "settings"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(filepath.Dir(filepath.Dir(task)), "settings", "api.json"), []byte(`{"apiKey":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}

	roots := (Adapter{}).Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("roots=%+v", roots)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 10 || evs[0].Output != 4 || evs[0].CacheRead != 2 || evs[0].CacheCreate != 1 {
		t.Fatalf("row1 %+v", evs[0])
	}
	if evs[0].Vendor != "anthropic" {
		t.Fatalf("vendor=%s", evs[0].Vendor)
	}
	for _, e := range evs {
		if strings.Contains(e.RequestID+e.Model, secret) {
			t.Fatalf("leaked %+v", e)
		}
	}
}

func TestMalformedJSONDoesNotPanic(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "tasks", "t")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "ui_messages.json"), []byte("{nope"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cline", Path: dir}, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}

func TestDiscoverWindowsAppData(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "AppData", "Roaming", "Code", "User", "globalStorage", extID, "tasks", "w")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "ui_messages.json"), []byte(`[{"type":"say","say":"api_req_started","ts":1,"text":"{\"tokensIn\":2,\"tokensOut\":1}"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := (Adapter{}).Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("windows AppData roots=%+v", roots)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(roots[0], func(e event.UsageEvent) { evs = append(evs, e) }, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 2 {
		t.Fatalf("%+v", evs)
	}
}
