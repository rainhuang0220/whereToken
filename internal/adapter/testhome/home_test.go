package testhome

import (
	"os"
	"path/filepath"
	"testing"
)

func TestDotDir(t *testing.T) {
	dir := t.TempDir()
	h := New(dir)
	want := filepath.Join(dir, ".kimi-code")
	if h.DotDir("kimi-code") != want {
		t.Fatalf("got %q", h.DotDir("kimi-code"))
	}
	if err := os.MkdirAll(want, 0o755); err != nil {
		t.Fatal(err)
	}
}
