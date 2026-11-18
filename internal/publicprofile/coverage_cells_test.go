package publicprofile

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestQuietDayWithoutCoverageIsUnknown(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	snap, err := Build(Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			ev("grok", "xai", now.Add(-time.Hour), 10, 0, 1),
		},
		SourceStatus: []SourceCoverage{{Source: "grok", Status: SourceFresh}},
	})
	if err != nil {
		t.Fatal(err)
	}
	series := quietTokenSeries(t, snap)
	empty := 0
	unknown := 0
	for _, state := range series.States {
		switch state {
		case CellEmpty:
			empty++
		case CellUnknown:
			unknown++
		}
	}
	if empty != 0 || unknown == 0 {
		t.Fatalf("empty=%d unknown=%d", empty, unknown)
	}
	if snap.Periods.All.Totals.Total.Value == nil || *snap.Periods.All.Totals.Total.Value == 0 {
		t.Fatalf("known usage became zero %+v", snap.Periods.All.Totals.Total)
	}
}

func TestConfirmedZeroDayStaysZero(t *testing.T) {
	now := time.Date(2026, 11, 18, 12, 0, 0, 0, time.UTC)
	snap, err := Build(Input{
		Now: now, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{
			ev("cursor", "cursor", time.Date(2026, 11, 18, 1, 0, 0, 0, time.UTC), 5, 0, 1),
		},
		SourceStatus: []SourceCoverage{{
			Source:             "cursor",
			Status:             SourceConfirmedZero,
			CoverageThroughUTC: "2026-11-17",
		}},
	})
	if err != nil {
		t.Fatal(err)
	}
	all := quietTokenSeries(t, snap)
	quiet := dayState(t, snap, all, "2026-11-10")
	if quiet != CellEmpty {
		t.Fatalf("audited quiet day %s", quiet)
	}
	openDay := dayState(t, snap, all, "2026-11-18")
	if openDay == CellEmpty {
		t.Fatal("a day after coverage was shown as zero")
	}
}

func quietTokenSeries(t *testing.T, snap Snapshot) Series {
	t.Helper()
	series := tokenSeries(snap)
	if series.Metric == "" && len(series.States) == 0 {
		t.Fatal("no token series")
	}
	return series
}

func dayState(t *testing.T, snap Snapshot, series Series, date string) string {
	t.Helper()
	for i, got := range snap.Activity.Dates {
		if got == date {
			return series.States[i]
		}
	}
	t.Fatalf("date %s not on the wall", date)
	return ""
}
