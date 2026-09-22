// Command syncprofileassets rebuilds a committed profile bundle from its
// existing privacy-safe profile.json. It refreshes static assets and previews
// without rescanning local usage or changing the snapshot's token data.
package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func main() {
	dir := flag.String("dir", "public-profile", "existing profile bundle")
	flag.Parse()

	snap, err := publicprofile.LoadFile(*dir)
	if err != nil {
		fail(err)
	}
	presentation := publicprofile.DefaultPresentation()
	if existing, err := publicprofile.ReadBundlePresentation(*dir); err == nil {
		presentation = existing
	}
	files, err := publicprofile.BundleWith(snap, presentation)
	if err != nil {
		fail(err)
	}
	for _, name := range publicprofile.GeneratedFiles {
		payload, ok := files[name]
		if !ok {
			continue
		}
		path := filepath.Join(*dir, name)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(path, payload, 0o644); err != nil {
			fail(err)
		}
	}
	for _, name := range publicprofile.RetiredGeneratedFiles {
		if err := os.Remove(filepath.Join(*dir, name)); err != nil && !os.IsNotExist(err) {
			fail(err)
		}
	}
	fmt.Println("refreshed assets in", *dir)
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "syncprofileassets:", err)
	os.Exit(1)
}
