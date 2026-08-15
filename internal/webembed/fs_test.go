package webembed

import "testing"

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
