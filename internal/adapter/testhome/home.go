package testhome

import (
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/adapter"
)

type home struct {
	root string
}

func New(root string) adapter.Home {
	return home{root: root}
}

func (h home) DotDir(name string) string {
	return filepath.Join(h.root, "."+name)
}

func (h home) XDGData(name string) string {
	return filepath.Join(h.root, ".local", "share", name)
}

func (h home) AppSupport(name string) string {
	return filepath.Join(h.root, "Library", "Application Support", name)
}

func (h home) XDGConfig(name string) string {
	return filepath.Join(h.root, ".config", name)
}

func (h home) AppData(name string) string {
	return filepath.Join(h.root, "AppData", "Roaming", name)
}
