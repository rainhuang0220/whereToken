package adapter

import (
	"os"
	"path/filepath"
)

func FirstFile(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err == nil && !st.IsDir() {
			return p
		}
	}
	return ""
}

func FirstDir(paths ...string) string {
	for _, p := range paths {
		if p == "" {
			continue
		}
		st, err := os.Stat(p)
		if err == nil && st.IsDir() {
			return p
		}
	}
	return ""
}

func VSCodeGlobalDB(home Home, product string) string {
	return FirstFile(
		filepath.Join(home.AppSupport(product), "User", "globalStorage", "state.vscdb"),
		filepath.Join(home.XDGConfig(product), "User", "globalStorage", "state.vscdb"),
		filepath.Join(home.AppData(product), "User", "globalStorage", "state.vscdb"),
	)
}

// VSCodeExtDir is User/globalStorage/<publisher.ext> for a VS Code-family product.
func VSCodeExtDir(home Home, product, extID string) string {
	return FirstDir(
		filepath.Join(home.AppSupport(product), "User", "globalStorage", extID),
		filepath.Join(home.XDGConfig(product), "User", "globalStorage", extID),
		filepath.Join(home.AppData(product), "User", "globalStorage", extID),
	)
}
