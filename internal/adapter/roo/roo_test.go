package roo

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
const extID = "RooVeterinaryInc.roo-cline"

func TestDiscoverAndParseMetrics(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "Library", "Application Support", "Code", "User", "globalStorage", extID, "tasks", "task-9")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	msgs := `[
	  {"type":"say","say":"api_req_started","ts":1787200000000,"text":"{\"tokensIn\":20,\"tokensOut\":3,\"cacheReads\":4,\"cacheWrites\":2,\"model\":\"claude-sonnet-4.6\"}"},
	  {"type":"say","say":"text","ts":1787200001000,"text":"PROMPT ` + secret + `"},
	  {"type":"say","say":"deleted_api_reqs","ts":1787200002000,"text":"{\"tokensIn\":99,\"tokensOut\":9}"}
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
	if len(evs) != 1 {
		t.Fatalf("Roo must not count deleted_api_reqs: events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 20 || evs[0].Output != 3 || evs[0].CacheRead != 4 || evs[0].CacheCreate != 2 {
		t.Fatalf("row %+v", evs[0])
	}
	if evs[0].Source != "roo" || evs[0].Vendor != "anthropic" {
		t.Fatalf("ids %+v", evs[0])
	}
	for _, e := range evs {
		if strings.Contains(e.RequestID+e.Model, secret) {
			t.Fatalf("leaked %+v", e)
		}
	}
}

func TestDiscoverWindowsAppData(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "AppData", "Roaming", "Code", "User", "globalStorage", extID, "tasks", "w")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(task, "ui_messages.json"), []byte(`[{"type":"say","say":"api_req_started","ts":1,"text":"{\"tokensIn\":1,\"tokensOut\":1}"}]`), 0o644); err != nil {
		t.Fatal(err)
	}
	roots := (Adapter{}).Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("windows AppData roots=%+v", roots)
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
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "roo", Path: dir}, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}
