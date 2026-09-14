package publicprofile

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/metric"
)

func shanghai() *time.Location {
	loc, err := time.LoadLocation("Asia/Shanghai")
	if err != nil {
		panic(err)
	}
	return loc
}

func ev(source, vendor string, at time.Time, miss, cache, out int64) event.UsageEvent {
	return event.UsageEvent{
		Source:     source,
		Vendor:     vendor,
		RequestID:  source + at.UTC().Format("20060102150405"),
		Timestamp:  at,
		Miss:       miss,
		CacheRead:  cache,
		Output:     out,
		Quality:    event.QualityAuthoritative,
		Derivation: event.DeriveRaw,
	}
}

func TestBuildEmptyIsUnavailableNotZero(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{Now: now, Loc: loc, Version: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	if snap.DataStatus != StatusUnavailable {
		t.Fatalf("status=%q", snap.DataStatus)
	}
	if snap.Periods.All.Totals.Total.Display != emDash || snap.Periods.All.Totals.Total.Value != nil {
		t.Fatalf("empty total=%+v", snap.Periods.All.Totals.Total)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
}

func TestBuildEmptyWithCostPublishesUnavailableCost(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	snap, err := Build(Input{Now: now, Loc: loc, Version: "dev", IncludeCost: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	cost := doc["periods"].(map[string]any)["all"].(map[string]any)["cost"].(map[string]any)
	if cost["status"] != StatusUnavailable || cost["display"] != emDash {
		t.Fatalf("cost=%v", cost)
	}
	if cost["usd_micro"] != nil || cost["priced_tokens"] != nil || cost["unpriced_tokens"] != nil {
		t.Fatalf("unavailable cost must use null numeric values: %v", cost)
	}
}

func TestBuildCostUsesContributingVendorVerificationDate(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	e := ev("kimi", "moonshot", now.Add(-time.Hour), 10, 0, 1)
	e.Model = "kimi-k3"
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc, IncludeCost: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	var doc map[string]any
	if err := json.Unmarshal(raw, &doc); err != nil {
		t.Fatal(err)
	}
	cost := doc["periods"].(map[string]any)["all"].(map[string]any)["cost"].(map[string]any)
	if cost["verified_at"] != "2026-08-20" {
		t.Fatalf("verified_at=%v", cost["verified_at"])
	}
}

func TestSnapshotIDStableWithoutDataChangeAndChangesWithData(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	baseEvent := ev("claude", "anthropic", now.Add(-time.Hour), 10, 0, 1)
	idOf := func(events []event.UsageEvent, at time.Time) string {
		t.Helper()
		snap, err := Build(Input{Events: events, Now: at, Loc: loc})
		if err != nil {
			t.Fatal(err)
		}
		raw, err := Marshal(snap)
		if err != nil {
			t.Fatal(err)
		}
		var doc map[string]any
		if err := json.Unmarshal(raw, &doc); err != nil {
			t.Fatal(err)
		}
		id, _ := doc["snapshot_id"].(string)
		return id
	}
	first := idOf([]event.UsageEvent{baseEvent}, now)
	second := idOf([]event.UsageEvent{baseEvent}, now.Add(time.Minute))
	if first == "" || first != second {
		t.Fatalf("same data ids %q %q", first, second)
	}
	changedEvent := baseEvent
	changedEvent.Miss++
	changed := idOf([]event.UsageEvent{changedEvent}, now)
	if changed == first {
		t.Fatalf("changed data retained id %q", first)
	}
}

func TestBuildUnknownSourceBecomesOther(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	poison := "/Users/rainhuang/secret"
	snap, err := Build(Input{
		Events: []event.UsageEvent{ev(poison, "anthropic", now.Add(-time.Hour), 10, 0, 1)},
		Now:    now, Loc: loc,
	})
	if err != nil {
		t.Fatal(err)
	}
	raw, _ := Marshal(snap)
	if strings.Contains(string(raw), poison) || strings.Contains(string(raw), "/Users/") {
		t.Fatalf("leaked path:\n%s", raw)
	}
	if len(snap.Periods.All.ByAgent) != 1 || snap.Periods.All.ByAgent[0].ID != AgentOtherID {
		t.Fatalf("agents=%+v", snap.Periods.All.ByAgent)
	}
}

func TestBuildUnknownModelBecomesOther(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	poison := `<script>alert(1)</script>`
	e := ev("claude", "anthropic", now.Add(-time.Hour), 10, 0, 1)
	e.Model = poison
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc, IncludeModels: true})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), poison) {
		t.Fatalf("leaked model: %s", raw)
	}
	if len(snap.Periods.All.ByModel) != 1 || snap.Periods.All.ByModel[0].ID != ModelOtherID {
		t.Fatalf("models=%+v", snap.Periods.All.ByModel)
	}
}

func TestBuildRejectsUnsafeOwnerFieldsAndRendersDisplayNameAsData(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	for _, owner := range []*Owner{
		{AvatarURL: "javascript:alert(1)"},
		{ProfileURL: "file:///etc/passwd"},
		{GitHubLogin: "../../../secret"},
	} {
		if _, err := Build(Input{Now: now, Loc: loc, Owner: owner}); err == nil {
			t.Fatalf("accepted unsafe owner %+v", owner)
		}
	}
	display := `<img src=x onerror=alert(1)>`
	snap, err := Build(Input{Now: now, Loc: loc, Owner: &Owner{DisplayName: display}})
	if err != nil {
		t.Fatal(err)
	}
	files, err := Bundle(snap)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(files["index.html"]), display) {
		t.Fatal("owner HTML was inlined into the page shell")
	}
	if !strings.Contains(string(files["assets/profile.js"]), ".textContent = owner.display_name") {
		t.Fatal("owner display name is not assigned through textContent")
	}
}

func TestBuildAllTimeMatchesAggregateAt(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{
		ev("claude", "anthropic", time.Date(2026, 9, 13, 10, 0, 0, 0, loc), 100, 20, 5),
		ev("kimi", "moonshot", time.Date(2026, 9, 12, 10, 0, 0, 0, loc), 50, 0, 10),
	}
	sum := metric.AggregateAt(events, nil, now, loc)
	snap, err := Build(Input{Events: events, Now: now, Loc: loc, Version: "dev"})
	if err != nil {
		t.Fatal(err)
	}
	got := *snap.Periods.All.Totals.Total.Value
	if got != sum.All.Total() {
		t.Fatalf("all=%d aggregate=%d", got, sum.All.Total())
	}
	if snap.Periods.All.HitRate.Display != metric.View(sum.All).HitRateText {
		t.Fatalf("hit %q vs %q", snap.Periods.All.HitRate.Display, metric.View(sum.All).HitRateText)
	}
	if snap.GeneratedAt == "" || snap.AsOfDate != "2026-09-13" {
		t.Fatalf("clock %+v", snap)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
}

func TestBuildPeriodWindowsUseCanonicalMergedRequests(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 14, 12, 0, 0, 0, loc)
	events := []event.UsageEvent{
		{Source: "claude", Vendor: "anthropic", RequestID: "cross-midnight", Timestamp: time.Date(2026, 9, 13, 23, 59, 0, 0, loc), Miss: 10, Quality: event.QualityAuthoritative},
		{Source: "claude", Vendor: "anthropic", RequestID: "cross-midnight", Timestamp: time.Date(2026, 9, 14, 0, 1, 0, 0, loc), Output: 5, Quality: event.QualityAuthoritative},
	}
	snap, err := Build(Input{Events: events, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	if got := *snap.Periods.All.Totals.Total.Value; got != 15 {
		t.Fatalf("all=%d", got)
	}
	if got := *snap.Periods.Today.Totals.Total.Value; got != 15 {
		t.Fatalf("today=%d; canonical request belongs wholly to its final local day", got)
	}
	if got := *snap.Periods.D7.Totals.Total.Value; got != 15 {
		t.Fatalf("7d=%d", got)
	}
	if snap.Periods.Today.CurrentStreak.Value == nil || *snap.Periods.Today.CurrentStreak.Value != 1 {
		t.Fatalf("today streak=%+v", snap.Periods.Today.CurrentStreak)
	}
	if snap.Periods.Today.Peak.Total.Value == nil || *snap.Periods.Today.Peak.Total.Value != 15 {
		t.Fatalf("today peak=%+v", snap.Periods.Today.Peak)
	}
}

func TestBuildReasoningNotInTotal(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	e := ev("grok", "xai", now.Add(-time.Hour), 10, 0, 5)
	e.Reasoning = 1000
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	if *snap.Periods.All.Totals.Total.Value != 15 {
		t.Fatalf("total=%d", *snap.Periods.All.Totals.Total.Value)
	}
}

func TestLevelsUseVisibleWindowOnly(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	inWall := ev("claude", "anthropic", time.Date(2026, 9, 10, 10, 0, 0, 0, loc), 100, 0, 0)
	base, err := Build(Input{Events: []event.UsageEvent{inWall}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	old := ev("claude", "anthropic", time.Date(2025, 8, 1, 10, 0, 0, 0, loc), 9_000_000, 0, 0)
	withHist, err := Build(Input{Events: []event.UsageEvent{inWall, old}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	a := seriesBy(base, "all", SeriesAllID, MetricTokens)
	b := seriesBy(withHist, "all", SeriesAllID, MetricTokens)
	for i := range a.Levels {
		if a.Levels[i] != b.Levels[i] {
			t.Fatalf("wall-external history changed visible intensity at %d: %d -> %d", i, a.Levels[i], b.Levels[i])
		}
	}
}

func TestMeasuredAuthoritativeZeroStaysAvailable(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	e := ev("claude", "anthropic", now.Add(-time.Hour), 0, 0, 0)
	snap, err := Build(Input{Events: []event.UsageEvent{e}, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
	row := snap.Periods.All.ByAgent[0]
	if row.Totals.Total.Status != StatusAvailable || row.Totals.Total.Value == nil || *row.Totals.Total.Value != 0 {
		t.Fatalf("measured zero became %+v", row.Totals.Total)
	}
	if row.Coverage.Tokens != StatusAvailable {
		t.Fatalf("coverage=%+v", row.Coverage)
	}
}

func TestOfflineCloudTokensAreUnavailableNotZero(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{
		ev("grok", "xai", now.Add(-time.Hour), 100, 0, 10),
		{
			Source: "cursor", Vendor: "anthropic", RequestID: "bubble-1",
			Timestamp: now.Add(-time.Hour), Quality: event.QualityDegraded, Derivation: event.DeriveRaw,
		},
	}
	snap, err := Build(Input{Events: events, Now: now, Loc: loc, Offline: true})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
	var cursor, grok Breakdown
	for _, row := range snap.Periods.All.ByAgent {
		switch row.ID {
		case "cursor":
			cursor = row
		case "grok":
			grok = row
		}
	}
	if cursor.ID != "cursor" {
		t.Fatal("missing cursor row")
	}
	if cursor.Totals.Total.Status != StatusUnavailable || cursor.Totals.Total.Value != nil || cursor.Totals.Total.Display != emDash {
		t.Fatalf("cursor tokens=%+v", cursor.Totals.Total)
	}
	if cursor.Share != emDash {
		t.Fatalf("cursor share=%q", cursor.Share)
	}
	if cursor.Requests.Value == nil || *cursor.Requests.Value != 1 || cursor.Requests.Status != StatusAvailable {
		t.Fatalf("cursor requests=%+v", cursor.Requests)
	}
	if cursor.Coverage.Tokens != StatusUnavailable || cursor.Coverage.Requests != StatusAvailable {
		t.Fatalf("coverage=%+v", cursor.Coverage)
	}
	if cursor.Coverage.Reason != ReasonAccountAPISkipped {
		t.Fatalf("reason=%q", cursor.Coverage.Reason)
	}
	if grok.Share != "100.0%" {
		t.Fatalf("grok share=%q (unavailable tokens must not enter denominator)", grok.Share)
	}
	if seriesBy(snap, "agent", "cursor", MetricTokens).ID != "" {
		t.Fatal("cursor token series must be omitted when tokens are unavailable")
	}
	req := seriesBy(snap, "agent", "cursor", MetricRequests)
	if req.ID != "cursor" {
		t.Fatal("cursor request series missing")
	}
	var reqSum int64
	for _, v := range req.Values {
		reqSum += v
	}
	if reqSum != 1 {
		t.Fatalf("cursor request series sum=%d", reqSum)
	}
	if err := ValidateProduction(snap, false); err == nil {
		t.Fatal("production validation must fail for skipped account API")
	} else if !strings.Contains(err.Error(), "Regenerate without --offline") {
		t.Fatalf("msg=%v", err)
	}
	if err := ValidateProduction(snap, true); err != nil {
		t.Fatal(err)
	}
}

func TestOnlineAccountAPICoverage(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 9, 13, 15, 0, 0, 0, loc)
	events := []event.UsageEvent{
		{Source: "cursor", Vendor: "anthropic", RequestID: "bubble", Timestamp: now.Add(-time.Hour), Quality: "", Derivation: event.DeriveRaw},
		{
			Source: "cursor", Vendor: "anthropic", RequestID: "api-1", Timestamp: now.Add(-time.Hour),
			Miss: 40, CacheRead: 200, CacheCreate: 10, Output: 5,
			Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		},
	}
	snap, err := Build(Input{Events: events, Now: now, Loc: loc})
	if err != nil {
		t.Fatal(err)
	}
	if err := Validate(snap); err != nil {
		t.Fatal(err)
	}
	var cursor Breakdown
	for _, row := range snap.Periods.All.ByAgent {
		if row.ID == "cursor" {
			cursor = row
		}
	}
	if cursor.Totals.Total.Value == nil || *cursor.Totals.Total.Value != 255 {
		t.Fatalf("cursor tokens=%+v", cursor.Totals.Total)
	}
	if cursor.Coverage.Tokens != StatusAvailable || cursor.Coverage.TokenSource != TokenSourceAccountAPI {
		t.Fatalf("coverage=%+v", cursor.Coverage)
	}
	if cursor.Coverage.TokenWindow == nil || cursor.Coverage.TokenWindow.From == nil {
		t.Fatalf("token window=%+v", cursor.Coverage.TokenWindow)
	}
	if cursor.Requests.Value == nil || *cursor.Requests.Value != 1 {
		t.Fatalf("requests=%+v", cursor.Requests)
	}
	if err := ValidateProduction(snap, false); err != nil {
		t.Fatal(err)
	}
}

func seriesBy(s Snapshot, dim, id, metricName string) Series {
	for _, ser := range s.Activity.Series {
		if ser.Dimension == dim && ser.ID == id && ser.Metric == metricName {
			return ser
		}
	}
	return Series{}
}
