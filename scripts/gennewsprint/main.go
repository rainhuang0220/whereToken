// Command gennewsprint bakes a printed-paper sheet from photoscanned
// Paper001 maps: height field, reconstructed normals, roughness, and
// desk-key lighting. It does not synthesize Fourier cockling or ship a
// grayscale overlay.
package main

import (
	"crypto/sha256"
	"flag"
	"fmt"
	"os"
	"path/filepath"
)

const (
	productionWidth = 1280
	displayWidthCSS = 560
	maxSurfaceBytes = 250 * 1024
)

func main() {
	root := flag.String("root", ".", "repository root")
	flag.Parse()

	vendor := filepath.Join(*root, "scripts", "gennewsprint", "vendor")
	surface, size, err := bakeFromVendor(
		filepath.Join(vendor, "Paper001_Color.jpg"),
		filepath.Join(vendor, "Paper001_Displacement.jpg"),
	)
	if err != nil {
		fail(err)
	}
	if len(surface) > maxSurfaceBytes {
		fail(fmt.Errorf("surface JPEG is %d bytes; budget is %d", len(surface), maxSurfaceBytes))
	}

	assetDir := filepath.Join(*root, "internal", "profilewebembed", "static", "assets")
	outputs := map[string][]byte{
		filepath.Join(assetDir, "newsprint-surface.jpg"):                        surface,
		filepath.Join(*root, "scripts", "gennewsprint", "material-swatch.html"): swatchHTML(),
	}
	for name, payload := range outputs {
		if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
			fail(err)
		}
		if err := os.WriteFile(name, payload, 0o644); err != nil {
			fail(err)
		}
		digest := sha256.Sum256(payload)
		fmt.Printf("wrote %s (%d bytes, %dx%d, sha256:%x)\n", name, len(payload), size.X, size.Y, digest)
	}
	for _, stale := range []string{
		filepath.Join(assetDir, "newsprint-surface.png"),
		filepath.Join(assetDir, "newsprint-fiber.svg"),
		filepath.Join(assetDir, "newsprint-fiber-b.svg"),
	} {
		if err := os.Remove(stale); err != nil && !os.IsNotExist(err) {
			fail(err)
		}
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gennewsprint:", err)
	os.Exit(1)
}
