package kilo

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
const extID = "kilocode.kilo-code"

func TestDiscoverAndParseMetrics(t *testing.T) {
	dir := t.TempDir()
	task := filepath.Join(dir, "Library", "Application Support", "Code", "User", "globalStorage", extID, "tasks", "task-k")
	if err := os.MkdirAll(task, 0o755); err != nil {
		t.Fatal(err)
	}
	msgs := `[
	  {"type":"say","say":"api_req_started","ts":1787200000000,"text":"{\"tokensIn\":15,\"tokensOut\":2,\"cacheReads\":3,\"cacheWrites\":1,\"model\":\"claude-sonnet-4.6\"}"},
	  {"type":"say","say":"text","ts":1787200001000,"text":"PROMPT ` + secret + `"}
	]`
	if err := os.WriteFile(filepath.Join(task, "ui_messages.json"), []byte(msgs), 0o644); err != nil {
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
		t.Fatalf("events=%d %+v", len(evs), evs)
	}
	if evs[0].Miss != 15 || evs[0].Output != 2 || evs[0].CacheRead != 3 || evs[0].CacheCreate != 1 {
		t.Fatalf("row %+v", evs[0])
	}
	if evs[0].Source != "kilo" || evs[0].Vendor != "anthropic" {
		t.Fatalf("ids %+v", evs[0])
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
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "kilo", Path: dir}, func(event.UsageEvent) {}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}
