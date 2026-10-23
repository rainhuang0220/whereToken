package hosted

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestKeptPreviousNamesRegression(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	device, _, _ := seedOwner(t)
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	first := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 20_000)},
	})
	if got := putProjection(t, h, device, first.raw); got["readme"] != "materialized" {
		t.Fatalf("first %+v", got)
	}
	puts := git.puts
	lower := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 19_000)},
	})
	kept := putProjection(t, h, device, lower.raw)
	if kept["kept"] != "previous" || kept["reason"] != "regression" || git.puts != puts {
		t.Fatalf("kept %+v puts %d -> %d", kept, puts, git.puts)
	}
}
