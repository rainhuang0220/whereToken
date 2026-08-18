package adapter

import (
	"bytes"
	"strings"
	"testing"
)

func TestReadHTTPBodyCapsSize(t *testing.T) {
	huge := bytes.Repeat([]byte("x"), MaxHTTPBody+4096)
	got, err := ReadHTTPBody(bytes.NewReader(huge))
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != MaxHTTPBody {
		t.Fatalf("len=%d want %d", len(got), MaxHTTPBody)
	}
	if strings.Contains(string(got), "secret-should-not-matter") {
		t.Fatal("unexpected")
	}
}
