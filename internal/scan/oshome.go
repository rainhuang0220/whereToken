package scan

import (
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

type osHome struct {
	home, xdg string
}

func RealHome() adapter.Home {
	if v := os.Getenv("WHERETOKEN_HOME"); v != "" {
		return testhome.New(v)
	}
	home, err := os.UserHomeDir()
	if err != nil {
		home = ""
	}
	return osHome{home: home, xdg: os.Getenv("XDG_DATA_HOME")}
}

func (h osHome) DotDir(name string) string {
	return filepath.Join(h.home, "."+name)
}

func (h osHome) XDGData(name string) string {
	if h.xdg != "" {
		return filepath.Join(h.xdg, name)
	}
	return filepath.Join(h.home, ".local", "share", name)
}

func (h osHome) AppSupport(name string) string {
	return filepath.Join(h.home, "Library", "Application Support", name)
}
