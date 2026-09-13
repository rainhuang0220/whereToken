package profilewebembed

import (
	"embed"
	"io/fs"
)

//go:embed all:static
var staticFS embed.FS

func Read(name string) ([]byte, error) {
	return staticFS.ReadFile("static/" + name)
}

func FS() fs.FS {
	sub, err := fs.Sub(staticFS, "static")
	if err != nil {
		return staticFS
	}
	return sub
}
