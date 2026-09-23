// Command gennewsprint bakes the Newsprint substrate from a photoscanned
// paper material. It does not synthesize the page as procedural cockling.
package main

import (
	"bytes"
	"crypto/sha256"
	"flag"
	"fmt"
	"image"
	"image/jpeg"
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
	colorPath := filepath.Join(vendor, "Paper001_Color.jpg")
	dispPath := filepath.Join(vendor, "Paper001_Displacement.jpg")
	surface, size, err := bakeFromVendor(colorPath, dispPath)
	if err == nil {
		if img, decErr := jpeg.Decode(bytes.NewReader(surface)); decErr == nil {
			rgb := imageToRGB(img)
			mean, std, edge := lumaStats(rgb, size.X, size.Y)
			fmt.Printf("sheet luma mean=%.1f std=%.2f edge=%.2f\n", mean, std, edge)
		}
	}
	if err != nil {
		fail(err)
	}
	if len(surface) > maxSurfaceBytes {
		fail(fmt.Errorf("surface JPEG is %d bytes; budget is %d", len(surface), maxSurfaceBytes))
	}

	dispImg, err := loadImage(dispPath)
	if err != nil {
		fail(err)
	}
	height, hsize, err := encodeHeightJPEG(dispImg)
	if err != nil {
		fail(err)
	}
	if len(height) > maxHeightBytes {
		fail(fmt.Errorf("height JPEG is %d bytes; budget is %d", len(height), maxHeightBytes))
	}

	assetDir := filepath.Join(*root, "internal", "profilewebembed", "static", "assets")
	outputs := map[string][]byte{
		filepath.Join(assetDir, "newsprint-surface.jpg"):                        surface,
		filepath.Join(assetDir, "newsprint-height.jpg"):                         height,
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
		fmt.Printf("wrote %s (%d bytes, sha256:%x)\n", name, len(payload), digest)
		_ = hsize
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

func imageToRGB(img image.Image) []float64 {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	out := make([]float64, w*h*3)
	i := 0
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			r, g, bl, _ := img.At(x, y).RGBA()
			out[i] = float64(r >> 8)
			out[i+1] = float64(g >> 8)
			out[i+2] = float64(bl >> 8)
			i += 3
		}
	}
	return out
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, "gennewsprint:", err)
	os.Exit(1)
}
