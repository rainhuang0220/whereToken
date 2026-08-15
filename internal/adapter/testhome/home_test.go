package testhome

import (
	"os"
	"path/filepath"
	"strings"
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

func TestLayoutUsesFakeHomeNotRealHome(t *testing.T) {
	dir := t.TempDir()
	h := New(dir)
	if got := h.XDGConfig("Trae CN"); got != filepath.Join(dir, ".config", "Trae CN") {
		t.Fatalf("XDGConfig=%q", got)
	}
	if got := h.AppData("Cursor"); got != filepath.Join(dir, "AppData", "Roaming", "Cursor") {
		t.Fatalf("AppData=%q", got)
	}
	if got := h.AppSupport("Trae"); got != filepath.Join(dir, "Library", "Application Support", "Trae") {
		t.Fatalf("AppSupport=%q", got)
	}
	for _, p := range []string{h.DotDir("claude"), h.XDGData("opencode"), h.XDGConfig("Trae"), h.AppSupport("Cursor"), h.AppData("Trae CN")} {
		if p != dir && !strings.HasPrefix(p, dir+string(os.PathSeparator)) {
			t.Fatalf("path %q is not under fake home %q", p, dir)
		}
	}
}
