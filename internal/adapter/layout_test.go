package adapter

import (
	"os"
	"path/filepath"
	"testing"
)

type stubHome struct{ root string }

func (h stubHome) DotDir(name string) string {
	return filepath.Join(h.root, "."+name)
}
func (h stubHome) XDGData(name string) string {
	return filepath.Join(h.root, ".local", "share", name)
}
func (h stubHome) XDGConfig(name string) string {
	return filepath.Join(h.root, ".config", name)
}
func (h stubHome) AppSupport(name string) string {
	return filepath.Join(h.root, "Library", "Application Support", name)
}
func (h stubHome) AppData(name string) string {
	return filepath.Join(h.root, "AppData", "Roaming", name)
}

func TestVSCodeGlobalDBPrefersMacThenLinuxThenWindows(t *testing.T) {
	dir := t.TempDir()
	h := stubHome{root: dir}
	if VSCodeGlobalDB(h, "Trae") != "" {
		t.Fatal("missing dirs must be skipped")
	}
	linux := filepath.Join(dir, ".config", "Trae", "User", "globalStorage")
	if err := os.MkdirAll(linux, 0o755); err != nil {
		t.Fatal(err)
	}
	linuxDB := filepath.Join(linux, "state.vscdb")
	if err := os.WriteFile(linuxDB, []byte("x"), 0o644); err != nil {
		t.Fatal(err)
	}
	if got := VSCodeGlobalDB(h, "Trae"); got != linuxDB {
		t.Fatalf("got %q", got)
	}
}
