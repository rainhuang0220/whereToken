package card

import (
	"math"
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

func ts(loc *time.Location, y, m, d, hh int) time.Time {
	return time.Date(y, time.Month(m), d, hh, 0, 0, 0, loc)
}

func ev(source, vendor string, at time.Time, miss, cache, out int64) event.UsageEvent {
	return event.UsageEvent{
		Source:     source,
		Vendor:     vendor,
		RequestID:  source + at.UTC().Format("20060102150405") + itoa(miss),
		Timestamp:  at,
		Miss:       miss,
		CacheRead:  cache,
		Output:     out,
		Quality:    event.QualityAuthoritative,
		Derivation: event.DeriveRaw,
	}
}

func itoa(n int64) string {
	if n < 0 {
		return "n"
	}
	var b [20]byte
	i := len(b)
	if n == 0 {
		return "0"
	}
	for n > 0 {
		i--
		b[i] = byte('0' + n%10)
		n /= 10
	}
	return string(b[i:])
}

func viewOf(t *testing.T, events []event.UsageEvent, turns []event.TurnEvent, now time.Time, loc *time.Location) PublicCard {
	t.Helper()
	sum := metric.AggregateAt(events, turns, now, loc)
	return NewView(sum, StatusOf(sum), "dev")
}

func TestFormatCompact(t *testing.T) {
	t.Parallel()
	cases := []struct {
		n    int64
		want string
	}{
		{0, "0"},
		{1, "1"},
		{999, "999"},
		{1000, "1K"},
		{1500, "1.5K"},
		{1010, "1.01K"},
		{999_994, "999.99K"},
		{999_995, "1M"},
		{1_000_000, "1M"},
		{1_234_000, "1.23M"},
		{1_235_000, "1.24M"},
		{3_840_000_000, "3.84B"},
		{1_000_000_000_000, "1T"},
		{math.MaxInt64, "9223372.04T"},
		{-1, "—"},
		{math.MinInt64, "—"},
	}
	for _, c := range cases {
		t.Run(c.want+"_"+itoa(c.n), func(t *testing.T) {
			got := FormatCompact(c.n)
			if got != c.want {
				t.Fatalf("FormatCompact(%d)=%q want %q", c.n, got, c.want)
			}
			if strings.ContainsAny(got, "eE") && got != "—" {
				t.Fatalf("scientific notation: %q", got)
			}
		})
	}
}

func TestStatusOfUnavailableEmptyLedger(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	sum := metric.AggregateAt(nil, nil, now, loc)
	if got := StatusOf(sum); got != DataUnavailable {
		t.Fatalf("status=%q", got)
	}
	card := NewView(sum, StatusOf(sum), "dev")
	if card.DataStatus != DataUnavailable {
		t.Fatalf("status=%q", card.DataStatus)
	}
	if card.AllTimeTokens.Display != "—" || card.Last53WeeksTokens.Display != "—" {
		t.Fatalf("hero=%q %q", card.AllTimeTokens.Display, card.Last53WeeksTokens.Display)
	}
	if card.AllTimeTokens.Raw != 0 {
		t.Fatalf("raw=%d", card.AllTimeTokens.Raw)
	}
	if card.Cost.Display != "—" || card.Cost.Status != "unavailable" {
		t.Fatalf("cost=%+v", card.Cost)
	}
	if !card.Peak.Available && card.Peak.TokensDisplay != "—" {
		// peak unavailable is expected
	}
	if card.Peak.Available {
		t.Fatal("empty peak must be unavailable")
	}
	var unknown, future int
	for _, c := range card.Cells {
		switch c.State {
		case CellUnknown:
			unknown++
			if c.Level != 0 || c.TokensDisplay != "—" {
				t.Fatalf("unknown cell %+v", c)
			}
		case CellFuture:
			future++
		case CellEmpty, CellActive:
			t.Fatalf("unavailable painted measured %s on %s", c.State, c.Date)
		default:
			t.Fatalf("state=%q", c.State)
		}
	}
	if unknown == 0 {
		t.Fatal("expected unknown past cells")
	}
	if len(card.Cells) != 371 {
		t.Fatalf("cells=%d", len(card.Cells))
	}
}

func TestStatusOfRequestOnlyZeroIsAvailable(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{{
		Source:    "kimi",
		Vendor:    "moonshot",
		RequestID: "req-only",
		Timestamp: ts(loc, 2026, 8, 15, 10),
		Quality:   event.QualityAuthoritative,
	}}
	sum := metric.AggregateAt(events, nil, now, loc)
	if sum.All.Total() != 0 || sum.All.Requests != 1 {
		t.Fatalf("total=%d req=%d", sum.All.Total(), sum.All.Requests)
	}
	if StatusOf(sum) != DataAvailable {
		t.Fatalf("status=%q", StatusOf(sum))
	}
	card := NewView(sum, StatusOf(sum), "dev")
	if card.DataStatus != DataAvailable {
		t.Fatalf("status=%q", card.DataStatus)
	}
	if card.AllTimeTokens.Display != "0" {
		t.Fatalf("display=%q (must be measured zero, not —)", card.AllTimeTokens.Display)
	}
	for _, c := range card.Cells {
		if c.State == CellUnknown {
			t.Fatalf("request-only zero painted unknown on %s", c.Date)
		}
		if c.Date == "2026-08-15" && c.State != CellEmpty {
			t.Fatalf("today=%+v", c)
		}
	}
}

func TestStatusOfDegradedContributingIsPartial(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	e := ev("claude", "anthropic", ts(loc, 2026, 8, 15, 10), 100, 0, 10)
	e.Quality = event.QualityDegraded
	sum := metric.AggregateAt([]event.UsageEvent{e}, nil, now, loc)
	if StatusOf(sum) != DataPartial {
		t.Fatalf("status=%q", StatusOf(sum))
	}
}

func TestNewViewOneDayLocalBucketLevelStreakPeak(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 10), 100, 0, 0)}
	card := viewOf(t, events, nil, now, loc)
	if card.SchemaVersion != 1 || card.Version != "dev" {
		t.Fatalf("meta %+v", card)
	}
	if card.AsOfDate != "2026-08-15" {
		t.Fatalf("as_of=%s", card.AsOfDate)
	}
	if card.Range.WeekStart != "monday" {
		t.Fatalf("week=%s", card.Range.WeekStart)
	}
	if card.Range.WindowFrom != "2025-08-11" || card.Range.WindowTo != "2026-08-15" {
		t.Fatalf("range=%+v", card.Range)
	}
	if card.AllTimeTokens.Raw != 100 || card.AllTimeTokens.Display != "100" {
		t.Fatalf("all=%+v", card.AllTimeTokens)
	}
	if card.CurrentStreakDays != 1 || card.LongestStreakDays != 1 {
		t.Fatalf("streak %d %d", card.CurrentStreakDays, card.LongestStreakDays)
	}
	if !card.Peak.Available || card.Peak.Date != "2026-08-15" || card.Peak.TokensRaw != 100 {
		t.Fatalf("peak %+v", card.Peak)
	}
	if !card.Peak.VisibleInWall {
		t.Fatal("peak should be in wall")
	}
	var hit *Cell
	for i := range card.Cells {
		if card.Cells[i].Date == "2026-08-15" {
			hit = &card.Cells[i]
		}
	}
	if hit == nil {
		t.Fatal("missing today cell")
	}
	if hit.State != CellActive || hit.Level != 2 || hit.TokensRaw != 100 {
		t.Fatalf("today %+v (single positive day is Q1==max → 2)", *hit)
	}
}

func TestNewViewAlways371CellsMondayFirst(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15) // Wednesday
	card := viewOf(t, nil, nil, now, loc)
	if len(card.Cells) != 371 {
		t.Fatalf("cells=%d", len(card.Cells))
	}
	if card.Cells[0].Date != card.Range.WindowFrom {
		t.Fatalf("first=%s from=%s", card.Cells[0].Date, card.Range.WindowFrom)
	}
	start, err := time.Parse("2006-01-02", card.Cells[0].Date)
	if err != nil {
		t.Fatal(err)
	}
	if start.Weekday() != time.Monday {
		t.Fatalf("first weekday=%s", start.Weekday())
	}
	last, err := time.Parse("2006-01-02", card.Cells[370].Date)
	if err != nil {
		t.Fatal(err)
	}
	if last.Weekday() != time.Sunday {
		t.Fatalf("last weekday=%s", last.Weekday())
	}
	if last.Sub(start) != 370*24*time.Hour {
		t.Fatalf("span=%s", last.Sub(start))
	}
	// Wednesday: Thu–Sun of the current week are future.
	var future []string
	for _, c := range card.Cells {
		if c.State == CellFuture {
			future = append(future, c.Date)
		}
	}
	want := []string{"2026-09-10", "2026-09-11", "2026-09-12", "2026-09-13"}
	if strings.Join(future, ",") != strings.Join(want, ",") {
		t.Fatalf("future=%v want %v", future, want)
	}
}

func TestNewView53WeekTotalsIgnoreHistoryOutsideWindow(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	events := []event.UsageEvent{
		ev("claude", "anthropic", ts(loc, 2025, 8, 1, 10), 5_000_000, 0, 0), // before WindowFrom 2025-09-08
		ev("claude", "anthropic", ts(loc, 2026, 9, 9, 10), 100, 0, 0),
		ev("claude", "anthropic", ts(loc, 2026, 3, 12, 10), 9_000_000, 0, 0), // in window, peak
	}
	card := viewOf(t, events, nil, now, loc)
	if card.AllTimeTokens.Raw != 14_000_100 {
		t.Fatalf("all=%d", card.AllTimeTokens.Raw)
	}
	if card.Last53WeeksTokens.Raw != 9_000_100 {
		t.Fatalf("53w=%d (must not include 2025-08-01)", card.Last53WeeksTokens.Raw)
	}
	if card.ActiveDays53Weeks != 2 {
		t.Fatalf("active=%d", card.ActiveDays53Weeks)
	}
	if card.Peak.Date != "2026-03-12" || !card.Peak.VisibleInWall {
		t.Fatalf("peak %+v", card.Peak)
	}
	// Historical day still participates in intensity: 100 is the lowest of 3 positives.
	var today, old *Cell
	for i := range card.Cells {
		switch card.Cells[i].Date {
		case "2026-09-09":
			today = &card.Cells[i]
		case "2025-08-01":
			t.Fatal("pre-window day must not appear as a cell date outside the 371 grid")
		}
	}
	if today == nil || today.State != CellActive {
		t.Fatalf("today %+v", today)
	}
	_ = old
	// 100 <= Q1 of {100, 5e6, 9e6} → level 1, not recalculated from visible-only {100, 9e6}.
	if today.Level != 1 {
		t.Fatalf("today level=%d want 1 (history must affect quartiles)", today.Level)
	}
}

func TestNewViewIntensityBoundaries(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 20, 12)
	// 8 positive days: 10,20,30,40,50,60,70,80
	// n=8; q1=20, q2=40, q3=60
	var events []event.UsageEvent
	vals := []int64{10, 20, 30, 40, 50, 60, 70, 80}
	for i, v := range vals {
		events = append(events, ev("kimi", "moonshot", ts(loc, 2026, 8, 10+i, 10), v, 0, 0))
	}
	card := viewOf(t, events, nil, now, loc)
	wantLevel := map[string]int{
		"2026-08-10": 1,
		"2026-08-11": 1,
		"2026-08-12": 2,
		"2026-08-13": 2,
		"2026-08-14": 3,
		"2026-08-15": 3,
		"2026-08-16": 4,
		"2026-08-17": 4,
	}
	for _, c := range card.Cells {
		if lv, ok := wantLevel[c.Date]; ok {
			if c.Level != lv || c.State != CellActive {
				t.Fatalf("%s level=%d state=%s want %d active", c.Date, c.Level, c.State, lv)
			}
		}
	}
}

func TestNewViewIntensityQ1EqualsMax(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 13, 10), 7, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 14, 10), 7, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 10), 7, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	for _, date := range []string{"2026-08-13", "2026-08-14", "2026-08-15"} {
		for _, c := range card.Cells {
			if c.Date == date && c.Level != 2 {
				t.Fatalf("%s level=%d want 2", date, c.Level)
			}
		}
	}
}

func TestNewViewCurrentStreakTodayEmptyYesterdayActive(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 13, 10), 1, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 14, 10), 1, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if card.CurrentStreakDays != 2 {
		t.Fatalf("current=%d want 2 (yesterday+day before; today empty)", card.CurrentStreakDays)
	}
}

func TestNewViewCurrentStreakBothEmptyIsZero(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 10, 10), 1, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if card.CurrentStreakDays != 0 {
		t.Fatalf("current=%d", card.CurrentStreakDays)
	}
	if card.LongestStreakDays != 1 {
		t.Fatalf("longest=%d", card.LongestStreakDays)
	}
}

func TestNewViewPeakTieTakesLaterDate(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 10, 10), 50, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 12, 10), 50, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 11, 10), 40, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if card.Peak.Date != "2026-08-12" || card.Peak.TokensRaw != 50 {
		t.Fatalf("peak %+v", card.Peak)
	}
}

func TestNewViewUTCTimestampUsesLocalDate(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 16, 12)
	// 2026-08-15 16:00 UTC = 2026-08-16 00:00 Asia/Shanghai
	utc := time.Date(2026, 8, 15, 16, 0, 0, 0, time.UTC)
	events := []event.UsageEvent{ev("kimi", "moonshot", utc, 42, 0, 0)}
	card := viewOf(t, events, nil, now, loc)
	var hit *Cell
	for i := range card.Cells {
		if card.Cells[i].Date == "2026-08-16" {
			hit = &card.Cells[i]
		}
		if card.Cells[i].Date == "2026-08-15" && card.Cells[i].State == CellActive {
			t.Fatalf("UTC evening landed on UTC date: %+v", card.Cells[i])
		}
	}
	if hit == nil || hit.State != CellActive || hit.TokensRaw != 42 {
		t.Fatalf("shanghai day %+v", hit)
	}
}

func TestNewViewFutureCellsNotCountedActiveOrEmpty(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	card := viewOf(t, nil, nil, now, loc)
	if card.ActiveDays53Weeks != 0 {
		t.Fatalf("active=%d", card.ActiveDays53Weeks)
	}
	for _, c := range card.Cells {
		if c.Date > "2026-09-09" {
			if c.State != CellFuture {
				t.Fatalf("%s state=%s", c.Date, c.State)
			}
			if c.Level != 0 || c.TokensRaw != 0 {
				t.Fatalf("future usage %+v", c)
			}
		}
	}
}

func TestNewViewTopAgentsStableSortAndRest(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 10), 50, 0, 0),
		ev("claude", "anthropic", ts(loc, 2026, 8, 15, 11), 50, 0, 0),
		ev("codex", "openai", ts(loc, 2026, 8, 15, 12), 20, 0, 0),
		ev("grok", "xai", ts(loc, 2026, 8, 15, 13), 10, 0, 0),
		ev("cursor", "cursor", ts(loc, 2026, 8, 15, 14), 5, 0, 0),
		ev("gemini", "google", ts(loc, 2026, 8, 15, 15), 0, 0, 0), // total 0, dropped
	}
	events[len(events)-1].RequestID = "zero-req"
	sum := metric.AggregateAt(events, nil, now, loc)
	before := append([]metric.Slice(nil), sum.BySource...)
	card := NewView(sum, StatusOf(sum), "dev")
	if len(sum.BySource) != len(before) {
		t.Fatal("BySource length changed")
	}
	for i := range before {
		if sum.BySource[i].ID != before[i].ID || sum.BySource[i].Miss != before[i].Miss {
			t.Fatalf("input mutated at %d", i)
		}
	}
	if len(card.Agents) != 4 {
		t.Fatalf("agents=%d %+v", len(card.Agents), card.Agents)
	}
	if card.Agents[0].ID != "claude" || card.Agents[1].ID != "kimi" {
		t.Fatalf("tie break Total DESC, ID ASC: %s then %s", card.Agents[0].ID, card.Agents[1].ID)
	}
	if card.Agents[0].Label != "Claude Code" || card.Agents[1].Label != "Kimi Code" {
		t.Fatalf("labels %q %q", card.Agents[0].Label, card.Agents[1].Label)
	}
	if card.Agents[2].ID != "codex" {
		t.Fatalf("third=%s", card.Agents[2].ID)
	}
	rest := card.Agents[3]
	if !rest.Rest || rest.ID != "rest" || rest.Label != "Rest" {
		t.Fatalf("rest %+v", rest)
	}
	if rest.TokensRaw != 15 {
		t.Fatalf("rest tokens=%d", rest.TokensRaw)
	}
	all := card.AllTimeTokens.Raw
	if all != 135 {
		t.Fatalf("all=%d", all)
	}
	if card.Agents[0].ShareText != metric.FormatShare(50, all) {
		t.Fatalf("share=%q", card.Agents[0].ShareText)
	}
	if math.Abs(card.Agents[0].ShareRatio-50.0/135.0) > 1e-12 {
		t.Fatalf("ratio=%v", card.Agents[0].ShareRatio)
	}
	if math.Abs(rest.ShareRatio-15.0/135.0) > 1e-12 {
		t.Fatalf("rest ratio=%v", rest.ShareRatio)
	}
}

func TestNewViewPeakOutsideWindowNotVisibleInWall(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	events := []event.UsageEvent{
		ev("claude", "anthropic", ts(loc, 2025, 8, 1, 10), 99_000_000, 0, 0),
		ev("claude", "anthropic", ts(loc, 2026, 9, 9, 10), 100, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if !card.Peak.Available || card.Peak.Date != "2025-08-01" {
		t.Fatalf("peak %+v", card.Peak)
	}
	if card.Peak.VisibleInWall {
		t.Fatal("pre-window peak must not be outlined on the wall")
	}
	if card.Last53WeeksTokens.Raw != 100 {
		t.Fatalf("53w=%d", card.Last53WeeksTokens.Raw)
	}
}

func TestNewViewLongestStreakUsesHistoryOutsideWindow(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 9, 9, 15)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2025, 8, 1, 10), 1, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2025, 8, 2, 10), 1, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2025, 8, 3, 10), 1, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 9, 9, 10), 1, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if card.LongestStreakDays != 3 {
		t.Fatalf("longest=%d want 3 from pre-window days", card.LongestStreakDays)
	}
	if card.ActiveDays53Weeks != 1 {
		t.Fatalf("active=%d", card.ActiveDays53Weeks)
	}
}

func TestNewViewExactlyThreeAgentsHasNoRest(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("claude", "anthropic", ts(loc, 2026, 8, 15, 10), 30, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 11), 20, 0, 0),
		ev("codex", "openai", ts(loc, 2026, 8, 15, 12), 10, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if len(card.Agents) != 3 {
		t.Fatalf("agents=%d", len(card.Agents))
	}
	for _, a := range card.Agents {
		if a.Rest || a.ID == "rest" {
			t.Fatalf("unexpected rest %+v", a)
		}
	}
}

func TestNewViewCurrentStreakIncludesToday(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	events := []event.UsageEvent{
		ev("kimi", "moonshot", ts(loc, 2026, 8, 14, 10), 1, 0, 0),
		ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 10), 1, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if card.CurrentStreakDays != 2 {
		t.Fatalf("current=%d", card.CurrentStreakDays)
	}
}

func TestNewViewUnknownSourceBecomesOther(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	poison := "/Users/alice/src/secret-repo"
	events := []event.UsageEvent{
		ev(poison, "unknown", ts(loc, 2026, 8, 15, 10), 9, 0, 0),
		ev("窑工", "unknown", ts(loc, 2026, 8, 15, 11), 3, 0, 0),
		ev("claude", "anthropic", ts(loc, 2026, 8, 15, 12), 1, 0, 0),
	}
	card := viewOf(t, events, nil, now, loc)
	if len(card.Agents) != 2 {
		t.Fatalf("agents=%+v", card.Agents)
	}
	var other, claude *Agent
	for i := range card.Agents {
		switch card.Agents[i].ID {
		case agentOtherID:
			other = &card.Agents[i]
		case "claude":
			claude = &card.Agents[i]
		}
	}
	if other == nil || other.Label != agentOtherLabel || other.TokensRaw != 12 {
		t.Fatalf("other %+v", card.Agents)
	}
	if claude == nil || claude.Label != "Claude Code" {
		t.Fatalf("claude %+v", card.Agents)
	}
	blob := card.Agents[0].ID + card.Agents[0].Label + card.Agents[1].ID + card.Agents[1].Label
	for _, p := range []string{poison, "窑工", "/Users/", "alice"} {
		if strings.Contains(blob, p) {
			t.Fatalf("leaked %q in %+v", p, card.Agents)
		}
	}
}

func TestNewViewCostCompletePartialUnavailable(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)

	// Unpriced model → unavailable cost, never $0.
	unknown := ev("kimi", "moonshot", ts(loc, 2026, 8, 15, 10), 100, 0, 10)
	unknown.Model = "not-a-real-model"
	card := viewOf(t, []event.UsageEvent{unknown}, nil, now, loc)
	if card.Cost.Status != "unavailable" {
		t.Fatalf("status=%q", card.Cost.Status)
	}
	if card.Cost.Display != "—" || strings.Contains(card.Cost.Display, "$0") {
		t.Fatalf("display=%q", card.Cost.Display)
	}

	// Priced Claude opus is on the card.
	priced := ev("claude", "anthropic", ts(loc, 2026, 8, 15, 10), 1_000_000, 0, 0)
	priced.Model = "claude-opus-4.6"
	card = viewOf(t, []event.UsageEvent{priced}, nil, now, loc)
	if card.Cost.Status != "complete" {
		t.Fatalf("priced status=%q", card.Cost.Status)
	}
	if card.Cost.Display == "—" || card.Cost.Display == "" || strings.Contains(card.Cost.Display, "$0") {
		t.Fatalf("priced display=%q", card.Cost.Display)
	}

	mixed := []event.UsageEvent{priced, unknown}
	card = viewOf(t, mixed, nil, now, loc)
	if card.Cost.Status != "partial" {
		t.Fatalf("mixed status=%q", card.Cost.Status)
	}
	if card.Cost.Display == "—" || strings.Contains(card.Cost.Display, "$0") {
		t.Fatalf("partial display=%q", card.Cost.Display)
	}
}

func TestNewViewDoesNotCopyForbiddenFields(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	e := ev("claude", "anthropic", ts(loc, 2026, 8, 15, 10), 10, 0, 1)
	e.Workspace = "/Users/alice/src/secret-repo"
	e.SessionID = "sess_LIVE_SECRET_99"
	e.RequestID = "req_LIVE_SECRET_99"
	e.SourceRoot = "/Users/alice/.claude"
	e.Model = "claude-opus-4.6"
	e.Provider = "anthropic-direct"
	card := viewOf(t, []event.UsageEvent{e}, nil, now, loc)
	blob := card.Agents[0].ID + card.Agents[0].Label + card.Version + card.AsOfDate
	for _, c := range card.Cells {
		blob += c.Date + c.State + c.TokensDisplay
	}
	for _, a := range card.Agents {
		blob += a.ID + a.Label + a.ShareText
	}
	for _, poison := range []string{
		"/Users/alice", "secret-repo", "sess_LIVE_SECRET_99", "req_LIVE_SECRET_99",
		"claude-opus-4.6", "anthropic-direct",
	} {
		if strings.Contains(blob, poison) {
			t.Fatalf("DTO leaked %q", poison)
		}
	}
}

func TestNewViewReasoningNotAddedToTotal(t *testing.T) {
	t.Parallel()
	loc := shanghai()
	now := ts(loc, 2026, 8, 15, 12)
	e := ev("grok", "xai", ts(loc, 2026, 8, 15, 10), 10, 0, 5)
	e.Reasoning = 1000
	card := viewOf(t, []event.UsageEvent{e}, nil, now, loc)
	if card.AllTimeTokens.Raw != 15 {
		t.Fatalf("total=%d (reasoning must not add)", card.AllTimeTokens.Raw)
	}
}
