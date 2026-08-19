package kimi

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestDiscoverDedupesKimiSymlink(t *testing.T) {
	dir := t.TempDir()
	realDir := filepath.Join(dir, ".kimi-code")
	if err := os.Mkdir(realDir, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, ".kimi")
	if err := os.Symlink(realDir, link); err != nil {
		t.Skipf("symlink: %v", err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 {
		t.Fatalf("symlink of .kimi -> .kimi-code must be one root, got %d %+v", len(roots), roots)
	}
}

func TestParseSameMillisecondRecordsStayDistinct(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	wire := filepath.Join(sess, "wire.jsonl")
	body := `{"type":"usage.record","time":1700000000000,"model":"k3","usage":{"inputOther":10,"output":1,"inputCacheRead":0,"inputCacheCreation":0}}
{"type":"usage.record","time":1700000000000,"model":"k3","usage":{"inputOther":20,"output":2,"inputCacheRead":0,"inputCacheCreation":0}}
`
	if err := os.WriteFile(wire, []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "kimi", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].RequestID == evs[1].RequestID {
		t.Fatalf("same-ms rows must not share request id: %q", evs[0].RequestID)
	}
}

func TestParseUsageRecord(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "kimi")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "kimi", Path: root}, func(e event.UsageEvent) { evs = append(evs, e) }, func(te event.TurnEvent) { turns = append(turns, te) }); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Vendor != "moonshot" || evs[0].Source != "kimi" {
		t.Fatalf("%+v", evs[0])
	}
	if evs[0].Miss != 100 || evs[0].CacheRead != 900 || evs[0].Output != 10 {
		t.Fatalf("%+v", evs[0])
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
	if evs[0].SessionID != "session" {
		t.Fatalf("session=%q", evs[0].SessionID)
	}
}

func TestSessionScopedUsageIsNotSummed(t *testing.T) {
	dir := t.TempDir()
	sess := filepath.Join(dir, "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(sess, 0o755); err != nil {
		t.Fatal(err)
	}
	body := `{"type":"usage.record","usageScope":"turn","time":1,"model":"kimi-k3","usage":{"inputOther":10,"output":1}}
{"type":"usage.record","usageScope":"session","time":2,"model":"kimi-k3","usage":{"inputOther":99,"output":9}}
`
	if err := os.WriteFile(filepath.Join(sess, "wire.jsonl"), []byte(body), 0o644); err != nil {
		t.Fatal(err)
	}
	var evs []event.UsageEvent
	if err := (Adapter{}).Parse(adapter.SourceRoot{ID: "kimi", Path: dir}, func(e event.UsageEvent) {
		evs = append(evs, e)
	}, func(event.TurnEvent) {}); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 1 || evs[0].Miss != 10 {
		t.Fatalf("session-scoped cumulative must be skipped: %+v", evs)
	}
}

func TestDiscoverHonorsKimiCodeHome(t *testing.T) {
	dir := t.TempDir()
	home := filepath.Join(dir, "relocated")
	if err := os.Mkdir(home, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("KIMI_CODE_HOME", home)
	roots := (Adapter{}).Discover(testhome.New(t.TempDir()))
	if len(roots) != 1 || roots[0].Path != home {
		t.Fatalf("KIMI_CODE_HOME roots=%+v", roots)
	}
}
