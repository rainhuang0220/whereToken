package scan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestOsHomeHonorsXDGAndAppDataEnv(t *testing.T) {
	t.Setenv("WHERETOKEN_HOME", "")
	t.Setenv("XDG_DATA_HOME", "/tmp/xdg-data")
	t.Setenv("XDG_CONFIG_HOME", "/tmp/xdg-config")
	t.Setenv("APPDATA", "/tmp/appdata")
	h, ok := RealHome().(osHome)
	if !ok {
		t.Fatalf("RealHome type %T", RealHome())
	}
	if h.XDGData("opencode") != filepath.Join("/tmp/xdg-data", "opencode") {
		t.Fatalf("XDGData=%q", h.XDGData("opencode"))
	}
	if h.XDGConfig("Trae") != filepath.Join("/tmp/xdg-config", "Trae") {
		t.Fatalf("XDGConfig=%q", h.XDGConfig("Trae"))
	}
	if h.AppData("Cursor") != filepath.Join("/tmp/appdata", "Cursor") {
		t.Fatalf("AppData=%q", h.AppData("Cursor"))
	}
}

func TestRealHomeOverrideIsFake(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("WHERETOKEN_HOME", dir)
	h := RealHome()
	if got := h.DotDir("claude"); got != filepath.Join(dir, ".claude") {
		t.Fatalf("got %q", got)
	}
}

func TestExtraHomesFromEnv(t *testing.T) {
	a := t.TempDir()
	b := t.TempDir()
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", a+string(os.PathListSeparator)+b)
	homes := extraHomes()
	if len(homes) != 2 {
		t.Fatalf("homes=%d", len(homes))
	}
	if homes[0].DotDir("x") != filepath.Join(a, ".x") {
		t.Fatalf("first=%q", homes[0].DotDir("x"))
	}
	if homes[1].DotDir("x") != filepath.Join(b, ".x") {
		t.Fatalf("second=%q", homes[1].DotDir("x"))
	}
}
