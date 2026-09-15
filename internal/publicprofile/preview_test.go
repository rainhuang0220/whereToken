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
	for i := range snap.Activity.Series {
		ser := &snap.Activity.Series[i]
		if ser.Dimension != "all" || (ser.Metric != MetricTokens && ser.Metric != "") {
			continue
		}
		for level := 1; level <= 5; level++ {
			ser.States[level-1] = CellActive
			ser.Levels[level-1] = level
			ser.Values[level-1] = int64(level)
		}
	}
	var light, dark bytes.Buffer
	if err := RenderPreview(&light, snap, ThemeLight); err != nil {
		t.Fatal(err)
	}
	if err := RenderPreview(&dark, snap, ThemeDark); err != nil {
		t.Fatal(err)
	}
	ls, ds := light.String(), dark.String()
	if !strings.Contains(ls, `viewBox="0 0 800 204"`) || !strings.Contains(ds, `viewBox="0 0 800 204"`) {
		t.Fatal("dimensions")
	}
	for _, forbidden := range []string{"AI coding profile", "Local public snapshot", "Public snapshot", "Explore profile", "→"} {
		if strings.Contains(ls, forbidden) || strings.Contains(ds, forbidden) {
			t.Fatalf("removed preview copy %q is still present", forbidden)
		}
	}
	if !strings.Contains(ls, "#F7F6F3") || !strings.Contains(ds, "#11100F") {
		t.Fatal("theme colors")
	}
	for _, color := range []string{"#EEEAE4", "#E4D4C1", "#D5AE86", "#C57E4A", "#A65331", "#71321F"} {
		if !strings.Contains(ls, color) {
			t.Fatalf("light heat color %s missing", color)
		}
	}
	for _, color := range []string{"#24211E", "#3A2B24", "#65402D", "#965837", "#C77845", "#F1A260"} {
		if !strings.Contains(ds, color) {
			t.Fatalf("dark heat color %s missing", color)
		}
	}
	if !strings.Contains(ls, "1.01K") || !strings.Contains(ls, "tokens") {
		t.Fatal("total token label")
	}
	if got := strings.Count(ls, `width="12.0" height="12.0"`); got != WallDays {
		t.Fatalf("wall cells=%d want=%d", got, WallDays)
	}
	if !strings.Contains(ls, `x="30.0" y="96.0" width="12.0"`) || !strings.Contains(ls, `x="758.0"`) || !strings.Contains(ls, `y="180.0"`) {
		t.Fatal("wall does not span the intended 740px width")
	}
	if !strings.Contains(ls, `metadata id="wheretoken-snapshot-id"`) || !strings.Contains(ds, snap.SnapshotID) {
		t.Fatal("snapshot metadata")
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
