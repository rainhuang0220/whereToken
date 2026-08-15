package scan

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestRunKimiFixture(t *testing.T) {
	dir := t.TempDir()
	dstDir := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dstDir, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	r := Run(testhome.New(dir), AllAdapters())
	if r.Summary.All.Total() != 1185 {
		t.Fatalf("all=%d", r.Summary.All.Total())
	}
	var kimi, moon int64
	for _, s := range r.Summary.BySource {
		if s.ID == "kimi" {
			kimi = s.Total()
		}
	}
	for _, s := range r.Summary.ByVendor {
		if s.ID == "moonshot" {
			moon = s.Total()
		}
	}
	if kimi != r.Summary.All.Total() || moon != r.Summary.All.Total() {
		t.Fatalf("kimi=%d moon=%d all=%d", kimi, moon, r.Summary.All.Total())
	}
}
