// Command gencarddemo renders the committed Vibe Coding Wall demo SVG from
// the synthetic fixture in internal/card. It never reads the maintainer HOME.
//
//	go run ./scripts/gencarddemo
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/rainhuang0220/whereToken/internal/card"
)

func main() {
	c := card.DemoCard()
	var buf bytes.Buffer
	if err := card.Render(&buf, c); err != nil {
		fmt.Fprintf(os.Stderr, "gencarddemo: %v\n", err)
		os.Exit(1)
	}
	out := buf.Bytes()
	paths := []string{
		filepath.Join("internal", "card", "testdata", "vibe-wall.golden.svg"),
		filepath.Join("docs", "media", "vibe-coding-wall-demo.svg"),
	}
	for _, p := range paths {
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			fmt.Fprintf(os.Stderr, "gencarddemo: %v\n", err)
			os.Exit(1)
		}
		if err := os.WriteFile(p, out, 0o644); err != nil {
			fmt.Fprintf(os.Stderr, "gencarddemo: %v\n", err)
			os.Exit(1)
		}
		fmt.Println("wrote", p)
	}
	fix := struct {
		Now          string `json:"now"`
		Location     string `json:"location"`
		Version      string `json:"version"`
		Seed         int64  `json:"seed"`
		LookbackDays int    `json:"lookback_days"`
		Peak         string `json:"peak"`
	}{
		Now:          card.DemoNow().Format("2006-01-02T15:04:05-07:00"),
		Location:     card.DemoLoc().String(),
		Version:      "dev",
		Seed:         7,
		LookbackDays: 420,
		Peak:         c.Peak.Date,
	}
	raw, err := json.MarshalIndent(fix, "", "  ")
	if err != nil {
		fmt.Fprintf(os.Stderr, "gencarddemo: %v\n", err)
		os.Exit(1)
	}
	fp := filepath.Join("internal", "card", "testdata", "vibe-wall.fixture.json")
	if err := os.WriteFile(fp, append(raw, '\n'), 0o644); err != nil {
		fmt.Fprintf(os.Stderr, "gencarddemo: %v\n", err)
		os.Exit(1)
	}
	fmt.Println("wrote", fp)
}
