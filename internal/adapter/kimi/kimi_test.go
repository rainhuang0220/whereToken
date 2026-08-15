package kimi

import (
	"path/filepath"
	"runtime"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestParseUsageRecord(t *testing.T) {
	_, file, _, _ := runtime.Caller(0)
	root := filepath.Join(filepath.Dir(file), "..", "..", "..", "testdata", "adapters", "kimi")
	var evs []event.UsageEvent
	var turns []event.TurnEvent
	a := Adapter{}
	if err := a.Parse(adapter.SourceRoot{ID: "kimi", Path: root}, func(e event.UsageEvent) { evs = append(evs, e) }, func(te event.TurnEvent) { turns = append(turns, te) }); err != nil {
		t.Fatal(err)
	}
	if len(evs) != 2 {
		t.Fatalf("events=%d", len(evs))
	}
	if evs[0].Vendor != "moonshot" || evs[0].Source != "kimi" {
		t.Fatalf("%+v", evs[0])
	}
	if evs[0].Miss != 100 || evs[0].CacheRead != 900 || evs[0].Output != 10 {
		t.Fatalf("%+v", evs[0])
	}
	if len(turns) != 1 {
		t.Fatalf("turns=%d", len(turns))
	}
}
