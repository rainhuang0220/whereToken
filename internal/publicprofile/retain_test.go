package publicprofile

import (
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestRetainedCursorStaysMeasuredWithoutDoubleCountingOrAZero(t *testing.T) {
	loc := shanghai()
	now := time.Date(2026, 10, 26, 12, 0, 0, 0, loc)
	local := ev("cursor", "cursor", now.Add(-2*time.Hour), 100, 0, 10)
	local.Quality = event.QualityDegraded
	retained := event.UsageEvent{
		Source: "cursor", RequestID: "retained:2026-10-23",
		Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Miss:      20, CacheRead: 30, CacheCreate: 4, Output: 6,
		Quality:     event.QualityDegraded,
		Derivation:  event.DeriveRetained,
		SkipRequest: true,
	}
	events := ApplyRetainedEvents([]event.UsageEvent{local}, []event.UsageEvent{retained})
	var tokens int64
	for _, e := range events {
		tokens += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	if tokens != 60 {
		t.Fatalf("tokens=%d, local rows were added to the observation", tokens)
	}
	snap, err := Build(Input{
		Events: events, Now: now, Loc: loc,
		Errors: []string{"cursor: 本机登录态已失效"},
	})
	if err != nil {
		t.Fatal(err)
	}
	var row Breakdown
	for _, agent := range snap.Periods.All.ByAgent {
		if agent.ID == "cursor" {
			row = agent
		}
	}
	if row.Totals.Total.Value == nil || *row.Totals.Total.Value != 60 {
		t.Fatalf("total=%+v", row.Totals.Total)
	}
	if row.Coverage.Tokens != StatusPartial || row.Coverage.Reason != "" || row.Coverage.TokenSource != TokenSourceAccountAPI {
		t.Fatalf("coverage=%+v", row.Coverage)
	}
	if row.Coverage.TokenWindow == nil || row.Coverage.TokenWindow.Label != "account usage" || row.Coverage.TokenWindow.To != "2026-10-23" {
		t.Fatalf("window=%+v", row.Coverage.TokenWindow)
	}
	if !measuredAgent(row) {
		t.Fatal("retained observation was treated as unmeasured")
	}
	prev := projectionWith("sha256:prev", StatusPartial, agentRow("cursor", 60, StatusAvailable, ""))
	if !ShouldReplaceProjection(prev, snap) {
		t.Fatal("retained partial cursor was treated as lost coverage")
	}
}

func TestSealedCursorKeepsVerifiedObservation(t *testing.T) {
	const observed int64 = 8_700_453_509
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	msg := "cursor: 账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥"
	local := ev("cursor", "cursor", now.Add(-time.Hour), 100, 0, 10)
	local.Quality = event.QualityDegraded
	local.Derivation = event.DeriveRaw
	if !AccountFetchFailed([]event.UsageEvent{local}, []string{msg}, "cursor") {
		t.Fatal("sealed session must count as a failed account fetch")
	}
	retained := event.UsageEvent{
		Source: "cursor", RequestID: "retained:2026-10-23",
		Timestamp:   time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Miss:        observed,
		Quality:     event.QualityDegraded,
		Derivation:  event.DeriveRetained,
		SkipRequest: true,
	}
	events := ApplyRetainedEvents([]event.UsageEvent{local}, []event.UsageEvent{retained})
	var tokens int64
	for _, e := range events {
		if e.Derivation == event.DeriveProviderAPI {
			t.Fatal("sealed path emitted a fresh account row")
		}
		tokens += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	if tokens != observed {
		t.Fatalf("tokens=%d want the retained observation, not a zero and not a double count", tokens)
	}
	snap, err := Build(Input{Events: events, Now: now, Loc: time.UTC, Errors: []string{msg}})
	if err != nil {
		t.Fatal(err)
	}
	var row Breakdown
	for _, agent := range snap.Periods.All.ByAgent {
		if agent.ID == "cursor" {
			row = agent
		}
	}
	if row.Totals.Total.Value == nil || *row.Totals.Total.Value != observed {
		t.Fatalf("total=%+v", row.Totals.Total)
	}
	if row.Coverage.Reason != "" || row.Coverage.Tokens == StatusUnavailable {
		t.Fatalf("coverage=%+v", row.Coverage)
	}
}

func TestSealedCursorWithoutHistoryIsUnavailableNotLoggedOut(t *testing.T) {
	now := time.Date(2026, 10, 31, 12, 0, 0, 0, time.UTC)
	local := event.UsageEvent{
		Source: "cursor", RequestID: "local", Timestamp: now,
		Quality: event.QualityDegraded, Derivation: event.DeriveRaw,
	}
	snap, err := Build(Input{
		Events: []event.UsageEvent{local}, Now: now, Loc: time.UTC,
		Errors: []string{"cursor: 账号用量不可用：Cursor 把会话封存在系统密钥中，本程序不读取该密钥"},
	})
	if err != nil {
		t.Fatal(err)
	}
	row := snap.Periods.All.ByAgent[0]
	if row.Coverage.Tokens != StatusUnavailable || row.Coverage.Reason != ReasonAPIFailed {
		t.Fatalf("coverage=%+v", row.Coverage)
	}
	if row.Coverage.Reason == ReasonAuthMissing {
		t.Fatal("sealed session was labeled as a missing login")
	}
}

func TestSnapshotSeriesWithAGapIsNotImportedAsZero(t *testing.T) {
	total := int64(10)
	miss := int64(10)
	zero := int64(0)
	snap := Snapshot{
		Periods: Periods{All: Period{ByAgent: []Breakdown{{
			ID: "cursor",
			Totals: Totals{
				Miss:        Component{Value: &miss, Status: StatusAvailable},
				CacheRead:   Component{Value: &zero, Status: StatusAvailable},
				CacheCreate: Component{Value: &zero, Status: StatusAvailable},
				Output:      Component{Value: &zero, Status: StatusAvailable},
				Total:       Component{Value: &total, Status: StatusAvailable},
			},
			Coverage: Coverage{Tokens: StatusAvailable},
		}}}},
		Activity: Activity{
			Dates:  []string{"2026-10-01", "2026-10-03"},
			Series: []Series{{ID: "cursor", Metric: MetricTokens, Values: []int64{4, 6}}},
		},
	}
	if _, ok := DaysFromSnapshot(snap, "cursor"); ok {
		t.Fatal("a gap in the series was imported")
	}
	snap.Activity.Dates = []string{"2026-10-01", "2026-10-02"}
	snap.Activity.Series[0].Values = []int64{4, 6}
	days, ok := DaysFromSnapshot(snap, "cursor")
	if !ok || len(days) != 2 {
		t.Fatalf("contiguous series ok=%v days=%d", ok, len(days))
	}
	var sum int64
	for _, d := range days {
		sum += d.Miss + d.CacheRead + d.CacheCreate + d.Output
	}
	if sum != 10 || days[0].Miss+days[1].Miss != 10 {
		t.Fatalf("allocation changed the observation: %+v", days)
	}
}

func TestApplyRetainedWithoutAnObservationDoesNotInventZero(t *testing.T) {
	local := ev("cursor", "cursor", time.Date(2026, 10, 26, 0, 0, 0, 0, time.UTC), 5, 0, 1)
	got := ApplyRetainedEvents([]event.UsageEvent{local}, nil)
	if len(got) != 1 || got[0].Miss != 5 {
		t.Fatalf("empty observation changed local events: %+v", got)
	}
}
