// Command genprofilecandidates renders palette-review SVGs from an existing
// public snapshot. It does not modify or publish the production bundle.
package main

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func main() {
	if len(os.Args) != 3 {
		fmt.Fprintln(os.Stderr, "usage: genprofilecandidates <profile.json> <output-dir>")
		os.Exit(2)
	}
	snapshot, err := publicprofile.LoadFile(os.Args[1])
	if err != nil {
		fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", err)
		os.Exit(1)
	}
	if err := os.MkdirAll(os.Args[2], 0o755); err != nil {
		fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", err)
		os.Exit(1)
	}
	candidates := []struct {
		name  string
		ramp  string
		theme publicprofile.Theme
	}{
		{name: "external-cobalt.svg", ramp: "external-cobalt-ramp.svg", theme: publicprofile.ThemeLightCobalt},
		{name: "external-magenta.svg", ramp: "external-magenta-ramp.svg", theme: publicprofile.ThemeLightMagenta},
		{name: "external-newsprint.svg", ramp: "external-newsprint-ramp.svg", theme: publicprofile.ThemeLightNewsprint},
	}
	for _, candidate := range candidates {
		path := filepath.Join(os.Args[2], candidate.name)
		file, err := os.Create(path)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", err)
			os.Exit(1)
		}
		renderErr := publicprofile.RenderPreview(file, snapshot, candidate.theme)
		closeErr := file.Close()
		if renderErr != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", renderErr)
			os.Exit(1)
		}
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", closeErr)
			os.Exit(1)
		}
		fmt.Println(path)

		rampPath := filepath.Join(os.Args[2], candidate.ramp)
		rampFile, err := os.Create(rampPath)
		if err != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", err)
			os.Exit(1)
		}
		renderErr = publicprofile.RenderRampStrip(rampFile, candidate.theme)
		closeErr = rampFile.Close()
		if renderErr != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", renderErr)
			os.Exit(1)
		}
		if closeErr != nil {
			fmt.Fprintf(os.Stderr, "genprofilecandidates: %v\n", closeErr)
			os.Exit(1)
		}
		fmt.Println(rampPath)
	}
}
