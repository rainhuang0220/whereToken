// Command genprofiledemo writes the committed synthetic public-profile bundle.
// It never reads HOME.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/card"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func main() {
	snap, err := publicprofile.Build(publicprofile.Input{
		Events:       card.DemoEvents(),
		Now:          card.DemoNow(),
		Loc:          card.DemoLoc(),
		Version:      "demo",
		PortraitSeed: "demo-v1",
	})
	if err != nil {
		fmt.Fprintf(os.Stderr, "genprofiledemo: %v\n", err)
		os.Exit(1)
	}
	files, err := publicprofile.Bundle(snap)
	if err != nil {
		fmt.Fprintf(os.Stderr, "genprofiledemo: %v\n", err)
		os.Exit(1)
	}
	root := filepath.Join("docs", "media", "public-profile-demo")
	for name, payload := range files {
		p := filepath.Join(root, name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "genprofiledemo: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(p, payload, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "genprofiledemo: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("wrote", p)
	}
}
