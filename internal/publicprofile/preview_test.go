package publicprofile

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestPreviewLightAndDarkShareStructure(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{
		Events: []event.UsageEvent{ev("claude", "anthropic", now.Add(-time.Hour), 1000, 0, 10)},
		Now:    now, Loc: loc, Version: "v0.7.2-dev",
	})
	if err != nil {
		t.Fatal(err)
	}
	var light, dark bytes.Buffer
	if err := RenderPreview(&light, snap, ThemeLight); err != nil {
		t.Fatal(err)
	}
	if err := RenderPreview(&dark, snap, ThemeDark); err != nil {
		t.Fatal(err)
	}
	ls, ds := light.String(), dark.String()
	if !strings.Contains(ls, `viewBox="0 0 800 248"`) || !strings.Contains(ds, `viewBox="0 0 800 248"`) {
		t.Fatal("dimensions")
	}
	if strings.Contains(ls, "#0b0f10") || strings.Contains(ls, "scanline") {
		t.Fatal("light still CRT")
	}
	if !strings.Contains(ls, "#F8FAFC") || !strings.Contains(ds, "#090B11") {
		t.Fatal("theme colors")
	}
	if strings.Contains(ls, "#bf4a16") || strings.Contains(ds, "#e85d04") {
		t.Fatal("legacy orange palette")
	}
	if !strings.Contains(ls, "Explore profile") {
		t.Fatal("cta")
	}
	if !strings.Contains(ls, "tokens tracked") {
		t.Fatal("tokens tracked label")
	}
	if strings.Contains(ls, "/Users/") {
		t.Fatal("path leak")
	}
}

func TestFromSnapshotParity(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{ev("claude", "anthropic", now.Add(-time.Hour), 100, 20, 5)}
	snap, err := Build(Input{Events: events, Now: now, Loc: loc, IncludeCost: true})
	if err != nil {
		t.Fatal(err)
	}
	if snap.Periods.All.Totals.Total.Value == nil {
		t.Fatal("total")
	}
	if *snap.Periods.All.Totals.Total.Value != 125 {
		t.Fatalf("total=%d", *snap.Periods.All.Totals.Total.Value)
	}
}
