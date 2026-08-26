package qwen

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

const secret = "sk-leak-fixture-SECRETVALUE99"

func TestDiscoverAndParseUsageLedger(t *testing.T) {
	dir := t.TempDir()
	usage := filepath.Join(dir, ".qwen", "usage")
	if err := os.MkdirAll(usage, 0o755); err != nil {
		t.Fatal(err)
	}
	body := strings.Join([]string{
		`{"schemaVersion":1,"id":"r1","timestamp":"2026-08-20T12:00:00.000Z","sessionId":"s1","model":"qwen3-coder-plus","inputTokens":80,"outputTokens":8,"cachedTokens":20,"thoughtsTokens":4,"totalTokens":88}`,
		`{nope`,
		`{"schemaVersion":1,"id":"r2","timestamp":"2026-08-20T12:00:01.000Z","sessionId":"s1","model":"qwen3-coder-plus","inputTokens":10,"outputTokens":2,"cachedTokens":0,"thoughtsTokens":0,"totalTokens":12}`,
	}, "\n") + "\n"
	if err := os.WriteFile(filepath.Join(usage, "token-usage-2026-08.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".qwen", "settings.json"), []byte(`{"apiKey":"`+secret+`"}`), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, ".qwen", "oauth_creds.json"), []byte(`{"access_token":"`+secret+`"}`), 0o600); err != nil {
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
	if evs[0].Miss != 60 || evs[0].CacheRead != 20 || evs[0].Output != 8 || evs[0].Reasoning != 4 {
		t.Fatalf("row1 %+v", evs[0])
	}
	if evs[0].Vendor != "alibaba" {
		t.Fatalf("vendor=%s", evs[0].Vendor)
	}
	for _, e := range evs {
		if strings.Contains(e.RequestID+e.Model, secret) {
			t.Fatalf("leaked %+v", e)
		}
	}
}

func TestDiscoverHonorsRuntimeDir(t *testing.T) {
	dir := t.TempDir()
	runtime := filepath.Join(dir, "isolated")
	if err := os.MkdirAll(filepath.Join(runtime, "usage"), 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("QWEN_RUNTIME_DIR", runtime)
	t.Setenv("QWEN_HOME", "")
	roots := (Adapter{}).Discover(testhome.New(t.TempDir()))
	if len(roots) != 1 || roots[0].Path != runtime {
		t.Fatalf("QWEN_RUNTIME_DIR roots=%+v", roots)
	}
}

func TestDiscoverEmpty(t *testing.T) {
	if roots := (Adapter{}).Discover(testhome.New(t.TempDir())); len(roots) != 0 {
		t.Fatalf("%+v", roots)
	}
}

func TestConfigDirWithoutLedgerIsNotUsage(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".qwen"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := (Adapter{}).Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("config dir should still be discovered: %+v", roots)
	}
}
