package scan

import (
	"os"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/minimax"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestCollectionDoesNotCallAnEmptyReadFresh(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	run := &collectionRun{at: now, now: func() time.Time { return now }}
	run.add("claude", 0, "")
	if len(run.checks) != 0 {
		t.Fatal("a missing install was recorded")
	}
	run.add("gemini", 1, "ABSENT")
	run.add("minimax", 1, "NO_EVENTS")
	run.add("codex", 1, "LOCAL_READ")
	run.add("grok", 1, "")
	if got := run.checks[0].Status; got != publicprofile.SourceUnknown || run.checks[0].Category != "ABSENT" {
		t.Fatalf("absent %+v", run.checks[0])
	}
	if run.checks[1].Status != publicprofile.SourceUnknown || run.checks[1].LastObservationAt != nil {
		t.Fatalf("empty read %+v", run.checks[1])
	}
	if run.checks[2].Status != publicprofile.SourceUnavailable || run.checks[2].Category != "LOCAL_READ" {
		t.Fatalf("read failure %+v", run.checks[2])
	}
	if run.checks[3].Status != publicprofile.SourceFresh || run.checks[3].LastObservationAt == nil {
		t.Fatalf("events %+v", run.checks[3])
	}
}

func TestMinimaxDirectoryWithoutLedgerIsNotFresh(t *testing.T) {
	home := testhome.New(t.TempDir())
	if err := os.MkdirAll(home.DotDir("minimax"), 0o755); err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	res := RunWithCollection(home, []adapter.Adapter{minimax.Adapter{}}, nil, func() time.Time { return now })
	if res.Collection == nil || len(res.Collection.Sources) != 1 {
		t.Fatalf("collection %+v", res.Collection)
	}
	check := res.Collection.Sources[0]
	if check.Source != "minimax" || check.Status != publicprofile.SourceUnknown || check.Category != "NO_EVENTS" {
		t.Fatalf("check %+v", check)
	}
	if check.LastObservationAt != nil {
		t.Fatal("empty directory was treated as an observation")
	}
}
