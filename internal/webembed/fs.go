package webembed

import (
	"embed"
	"io/fs"
)

//go:embed all:dist
var dist embed.FS

func FS() (fs.FS, bool) {
	sub, err := fs.Sub(dist, "dist")
	if err != nil {
		return nil, false
	}
	f, err := sub.Open("index.html")
	if err != nil {
		return nil, false
	}
	_ = f.Close()
	return sub, true
}
