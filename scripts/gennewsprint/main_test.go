package main

import (
	"bytes"
	"crypto/sha256"
	"testing"
)

func TestBakeSurfaceIsDeterministicForFixedSeed(t *testing.T) {
	first, err := bakeSurface(192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := bakeSurface(192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(first, second) {
		t.Fatalf("same seed produced different hashes: %x != %x", sha256.Sum256(first), sha256.Sum256(second))
	}
}

func TestBakeSurfaceChangesWithSeed(t *testing.T) {
	first, err := bakeSurface(192, materialSeed)
	if err != nil {
		t.Fatal(err)
	}
	second, err := bakeSurface(192, materialSeed+1)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(first, second) {
		t.Fatal("different seeds produced identical surface assets")
	}
}

func TestFiberUsesLitAnisotropicBumpInsteadOfDirectNoise(t *testing.T) {
	fiber := fiberSVG()
	for _, want := range [][]byte{[]byte("feDiffuseLighting"), []byte(`baseFrequency="0.075 0.42"`), []byte(`stitchTiles="stitch"`)} {
		if !bytes.Contains(fiber, want) {
			t.Fatalf("fiber asset missing %q", want)
		}
	}
}
