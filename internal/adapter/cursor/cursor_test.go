package cursor

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestDiscoverCursorDotDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, ".cursor"), 0o755); err != nil {
		t.Fatal(err)
	}
	roots := Adapter{}.Discover(testhome.New(dir))
	if len(roots) != 1 || roots[0].ID != "cursor" {
		t.Fatalf("roots=%v", roots)
	}
}

func TestParseEmitsNothing(t *testing.T) {
	var n int
	err := (Adapter{}).Parse(adapter.SourceRoot{ID: "cursor", Path: t.TempDir()}, func(event.UsageEvent) {
		n++
	}, func(event.TurnEvent) {})
	if err != nil {
		t.Fatal(err)
	}
	if n != 0 {
		t.Fatalf("emitted %d events", n)
	}
}
