package publicprofile

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/vendor"
)

// Every allowlisted provider must survive projection even below rank eight.
// Unknown routing stays separate from the sanitized bucket for unrecognized ids.
func TestBuildKeepsAllVendorAccountingAndSeries(t *testing.T) {
	now := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	ids := append(vendor.KnownIDs(), "unrecognized-private-provider", "another-private-provider")
	var events []event.UsageEvent
	want := map[string]int64{}
	for i, id := range ids {
		n := int64(len(ids) - i)
		events = append(events, ev("cursor", id, now.Add(time.Duration(i)*time.Second), n, 2*n, 3*n))
		if i >= len(vendor.KnownIDs()) {
			id = VendorOtherID
		}
		want[id] += 6 * n
	}
	snap, err := Build(Input{Events: events, Now: now.Add(time.Hour), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	seen := map[string]bool{}
	for _, row := range snap.Periods.All.ByVendor {
		if seen[row.ID] {
			t.Fatalf("duplicate vendor %q", row.ID)
		}
		seen[row.ID] = true
		if row.Totals.Total.Value == nil || *row.Totals.Total.Value != want[row.ID] {
			t.Fatalf("vendor %q total=%+v want=%d", row.ID, row.Totals.Total, want[row.ID])
		}
	}
	for id := range want {
		if !seen[id] {
			t.Fatalf("missing vendor %q below rank eight", id)
		}
	}
	for _, metric := range []string{MetricTokens, MetricRequests} {
		got := map[string]bool{}
		for _, series := range snap.Activity.Series {
			if series.Dimension != "vendor" || series.Metric != metric {
				continue
			}
			if got[series.ID] {
				t.Fatalf("duplicate %s series %q", metric, series.ID)
			}
			got[series.ID] = true
		}
		for id := range want {
			if !got[id] {
				t.Fatalf("missing %s vendor series %q", metric, id)
			}
		}
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
}
