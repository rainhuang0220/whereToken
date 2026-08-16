package webembed

import (
	"io"
	"strings"
	"testing"
)

func TestFSHasIndexHTML(t *testing.T) {
	fsys, ok := FS()
	if !ok {
		t.Fatal("embed missing")
	}
	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
}

func TestEmbedDoesNotLoadGoogleFonts(t *testing.T) {
	fsys, ok := FS()
	if !ok {
		t.Fatal("embed missing")
	}
	f, err := fsys.Open("index.html")
	if err != nil {
		t.Fatal(err)
	}
	defer f.Close()
	raw, err := io.ReadAll(f)
	if err != nil {
		t.Fatal(err)
	}
	s := string(raw)
	if strings.Contains(s, "fonts.googleapis.com") || strings.Contains(s, "fonts.gstatic.com") {
		t.Fatalf("go install stub must stay local-first: %s", s)
	}
}
