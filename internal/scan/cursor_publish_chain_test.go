package scan

import (
	"database/sql"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/report"
)

const chainObserved int64 = 8_700_453_509

func TestCursorPublishChain(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	loc := time.FixedZone("CST", 8*3600)
	day := time.Date(2026, 10, 31, 18, 0, 0, 0, loc)
	nextDay := time.Date(2026, 11, 1, 9, 0, 0, 0, loc)
	published := day.Add(-6 * time.Hour)

	t.Run("new user has requests and no account total", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, false)
		res := scanChain(t, home)
		assertSealedScan(t, res)
		if saw := fullScan(res); !saw {
			t.Fatal("expected a full file reread")
		}
		events := retainChain(t, testhome.New(home), res)
		assertNoProviderAPI(t, events)
		if retainedSum(t, testhome.New(home), "cursor") != 0 {
			t.Fatal("a new user gained a cursor account observation")
		}
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		row := chainAgent(t, snap, "cursor")
		if row.Requests.Value == nil || *row.Requests.Value == 0 {
			t.Fatalf("requests=%+v", row.Requests)
		}
		if row.Totals.Total.Value != nil || row.Totals.Total.Status != publicprofile.StatusUnavailable {
			t.Fatalf("tokens=%+v", row.Totals.Total)
		}
		if row.Coverage.TokenSource == publicprofile.TokenSourceAccountAPI || row.Coverage.Reason != publicprofile.ReasonAPIFailed {
			t.Fatalf("coverage=%+v", row.Coverage)
		}
		if row.Totals.Total.Display == "0.00 M" || row.Totals.Total.Display == "0" {
			t.Fatalf("unavailable tokens displayed as %q", row.Totals.Total.Display)
		}
		rep := chainReport(t, events, res.Turns, res.Errors, day, loc)
		if rep.TotalM != "不可用" || rep.Requests == 0 || strings.Contains(rep.TotalM, "0.00") {
			t.Fatalf("report total_m=%q requests=%d", rep.TotalM, rep.Requests)
		}
		raw := chainJSON(t, rep)
		if !strings.Contains(raw, `"total_m": "不可用"`) || !strings.Contains(raw, `"requests":`) || strings.Contains(raw, "0.00 M") {
			t.Fatalf("json=%s", raw)
		}
		got := decideChain(snap, nil, nil, "", time.Time{}, day)
		if got.Code != publicprofile.CodeBlockedData || got.Label != "用量不可用" || got.Detail != "" || got.Publish {
			t.Fatalf("decision=%+v", got)
		}
	})

	t.Run("sealed session keeps the verified observation", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, true)
		seedChainObservation(t, testhome.New(home))
		res := scanChain(t, home)
		assertSealedScan(t, res)
		events := retainChain(t, testhome.New(home), res)
		assertNoProviderAPI(t, events)
		if sum := tokenSum(events, "cursor"); sum != chainObserved {
			t.Fatalf("cursor tokens=%d", sum)
		}
		if retainedSum(t, testhome.New(home), "cursor") != chainObserved {
			t.Fatal("ledger observation changed")
		}
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		row := chainAgent(t, snap, "cursor")
		assertRetainedCursor(t, snap, row)
		if row.Requests.Value == nil || *row.Requests.Value == 0 || *row.Requests.Value == *row.Totals.Total.Value {
			t.Fatalf("requests and tokens were collapsed: requests=%+v tokens=%+v", row.Requests, row.Totals.Total)
		}
		prev := buildChain(t, retainedOnly(chainObserved), nil, nil, day, loc)
		total := chainObserved
		got := decideChain(snap, &prev, &total, "2026-10-31", published, day)
		if got.Code == publicprofile.CodeBlockedData || got.Label == "覆盖变弱，保留上次" {
			t.Fatalf("retained cursor stayed blocked: %+v", got)
		}
		if got.Code != publicprofile.CodeSkippedDelta {
			t.Fatalf("decision=%+v", got)
		}
	})

	t.Run("fresh account row then a sealed session still publishes", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, true)
		seedChainObservation(t, testhome.New(home))
		res := scanChain(t, home)
		events := retainChain(t, testhome.New(home), res)
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		prev := buildChain(t, []event.UsageEvent{{
			Source: "cursor", RequestID: "cursor-api", Timestamp: time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
			Miss: chainObserved, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
		}}, nil, nil, day, loc)
		if prev.DataStatus != publicprofile.StatusAvailable {
			t.Fatalf("previous measurement status=%s", prev.DataStatus)
		}
		total := chainObserved
		got := decideChain(snap, &prev, &total, "2026-10-31", published, day)
		if got.Code != publicprofile.CodeSkippedDelta {
			t.Fatalf("sealed session kept the wall blocked: %+v", got)
		}
	})

	t.Run("cursor unavailable does not block grok and codex", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, false)
		writeChainGrok(t, home, 3_000_000)
		writeChainCodex(t, home, 2_000_000)
		res := scanChain(t, home)
		events := retainChain(t, testhome.New(home), res)
		if sum := tokenSum(events, "cursor"); sum != 0 {
			t.Fatalf("local cursor tokens entered the total: %d", sum)
		}
		if tokenSum(events, "grok") != 3_000_000 || tokenSum(events, "codex") != 2_000_000 {
			t.Fatalf("grok=%d codex=%d", tokenSum(events, "grok"), tokenSum(events, "codex"))
		}
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		row := chainAgent(t, snap, "cursor")
		if row.Coverage.TokenSource == publicprofile.TokenSourceAccountAPI || row.Totals.Total.Value != nil {
			t.Fatalf("cursor impersonated an account total: %+v", row)
		}
		if snap.Periods.All.Totals.Total.Value == nil || *snap.Periods.All.Totals.Total.Value != 5_000_000 {
			t.Fatalf("all=%+v", snap.Periods.All.Totals.Total)
		}
		prevEvents := []event.UsageEvent{
			{Source: "grok", RequestID: "old-grok", Timestamp: day.Add(-time.Hour), Miss: 1_000, Quality: event.QualityAuthoritative, Derivation: event.DeriveDerived},
			{Source: "codex", RequestID: "old-codex", Timestamp: day.Add(-time.Hour), Output: 1_000, Quality: event.QualityAuthoritative, Derivation: event.DeriveDerived},
		}
		prev := buildChain(t, prevEvents, nil, nil, day, loc)
		old := int64(2_000)
		got := decideChain(snap, &prev, &old, "2026-10-31", published, day)
		if got.Code == publicprofile.CodeBlockedData || !got.Publish || got.Code != publicprofile.CodePublished {
			t.Fatalf("decision=%+v", got)
		}
	})

	t.Run("retained cursor lets grok and codex publish", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, true)
		writeChainGrok(t, home, 3_000_000)
		writeChainCodex(t, home, 2_000_000)
		seedChainObservation(t, testhome.New(home))
		res := scanChain(t, home)
		events := retainChain(t, testhome.New(home), res)
		if sum := tokenSum(events, "cursor"); sum != chainObserved {
			t.Fatalf("cursor tokens=%d", sum)
		}
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		assertRetainedCursor(t, snap, chainAgent(t, snap, "cursor"))
		want := chainObserved + 5_000_000
		if snap.Periods.All.Totals.Total.Value == nil || *snap.Periods.All.Totals.Total.Value != want {
			t.Fatalf("all=%+v", snap.Periods.All.Totals.Total)
		}
		prev := buildChain(t, retainedOnly(chainObserved), nil, nil, day, loc)
		total := chainObserved
		got := decideChain(snap, &prev, &total, "2026-10-31", published, day)
		if got.Code == publicprofile.CodeBlockedData || !got.Publish || got.Code != publicprofile.CodePublished {
			t.Fatalf("decision=%+v", got)
		}
		if publicprofile.FormatCompact(chainObserved) == publicprofile.FormatCompact(want) {
			t.Fatal("fixture did not cross a visible tick")
		}
	})

	t.Run("retained cursor rolls the local date", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, true)
		seedChainObservation(t, testhome.New(home))
		res := scanChain(t, home)
		events := retainChain(t, testhome.New(home), res)
		snap := buildChain(t, events, res.Turns, res.Errors, nextDay, loc)
		row := chainAgent(t, snap, "cursor")
		assertRetainedCursor(t, snap, row)
		if row.Coverage.TokenWindow.Label == "53w account usage" || !(row.Coverage.TokenWindow.To < snap.AsOfDate) {
			t.Fatalf("window=%+v as_of=%s", row.Coverage.TokenWindow, snap.AsOfDate)
		}
		prev := buildChain(t, retainedOnly(chainObserved), nil, nil, day, loc)
		total := chainObserved
		got := decideChain(snap, &prev, &total, "2026-10-31", published, nextDay)
		if got.Code != publicprofile.CodeDateRollover || !got.Publish {
			t.Fatalf("decision=%+v", got)
		}
	})

	t.Run("same day visible growth waits out the hour and then publishes", func(t *testing.T) {
		home := t.TempDir()
		writeChainCursorDB(t, home, true)
		writeChainGrok(t, home, 3_000_000)
		writeChainCodex(t, home, 2_000_000)
		seedChainObservation(t, testhome.New(home))
		res := scanChain(t, home)
		events := retainChain(t, testhome.New(home), res)
		snap := buildChain(t, events, res.Turns, res.Errors, day, loc)
		prev := buildChain(t, retainedOnly(chainObserved), nil, nil, day, loc)
		total := chainObserved
		waiting := decideChain(snap, &prev, &total, "2026-10-31", day.Add(-20*time.Minute), day)
		if waiting.Code != publicprofile.CodeWaitingCooldown || !waiting.Publish {
			t.Fatalf("waiting=%+v", waiting)
		}
		ready := decideChain(snap, &prev, &total, "2026-10-31", published, day)
		if ready.Code != publicprofile.CodePublished || !ready.Publish {
			t.Fatalf("ready=%+v", ready)
		}
	})
}

func scanChain(t *testing.T, home string) Result {
	t.Helper()
	res := Run(testhome.New(home), Adapters(false))
	return res
}

func assertSealedScan(t *testing.T, res Result) {
	t.Helper()
	joined := strings.Join(res.Errors, "\n")
	if !strings.Contains(joined, "账号用量不可用") {
		t.Fatalf("errors=%v", res.Errors)
	}
	if strings.Contains(strings.ToLower(joined), "auth") || strings.Contains(joined, "未找到本机登录态") || strings.Contains(joined, "eyJ") {
		t.Fatalf("errors=%v", res.Errors)
	}
	found := false
	for i := range res.Summary.BySource {
		if res.Summary.BySource[i].ID == "cursor" {
			found = true
			if res.Summary.BySource[i].Quality == event.QualityAuthoritative {
				t.Fatal("a full file reread was labeled as a complete token measurement")
			}
			if res.Summary.BySource[i].Requests == 0 {
				t.Fatal("local requests were dropped")
			}
		}
	}
	if !found {
		t.Fatal("cursor row missing")
	}
}

func fullScan(res Result) bool {
	for _, d := range res.Deltas {
		if d.Mode == index.ModeFull {
			return true
		}
	}
	return false
}

func retainChain(t *testing.T, home adapter.Home, res Result) []event.UsageEvent {
	t.Helper()
	p, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	events := append([]event.UsageEvent(nil), res.Events...)
	next, err := lg.SupplementNarrowWindow(events)
	if err != nil {
		t.Fatal(err)
	}
	events = next
	seen := map[string]struct{}{}
	for _, msg := range res.Errors {
		id, _, ok := strings.Cut(msg, ":")
		if !ok {
			continue
		}
		if _, dup := seen[id]; dup {
			continue
		}
		seen[id] = struct{}{}
		if !publicprofile.AccountFetchFailed(events, res.Errors, id) {
			continue
		}
		got, ok, err := lg.LatestRetainedEvents(id)
		if err != nil {
			t.Fatal(err)
		}
		if ok {
			events = publicprofile.ApplyRetainedEvents(events, got)
		}
	}
	return events
}

func seedChainObservation(t *testing.T, home adapter.Home) {
	t.Helper()
	p, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	err = lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), retainedOnly(chainObserved))
	if err != nil {
		t.Fatal(err)
	}
}

func retainedOnly(total int64) []event.UsageEvent {
	return []event.UsageEvent{{
		Source: "cursor", RequestID: "retained:2026-10-23",
		Timestamp:   time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Miss:        total,
		Quality:     event.QualityDegraded,
		Derivation:  event.DeriveRetained,
		SkipRequest: true,
	}}
}

func retainedSum(t *testing.T, home adapter.Home, source string) int64 {
	t.Helper()
	p, err := ledger.PathFor(home)
	if err != nil {
		t.Fatal(err)
	}
	lg, err := ledger.Open(p)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	got, ok, err := lg.LatestRetainedEvents(source)
	if err != nil {
		t.Fatal(err)
	}
	if !ok {
		return 0
	}
	return tokenSum(got, source)
}

func buildChain(t *testing.T, events []event.UsageEvent, turns []event.TurnEvent, errs []string, now time.Time, loc *time.Location) publicprofile.Snapshot {
	t.Helper()
	snap, err := publicprofile.Build(publicprofile.Input{
		Events: events, Turns: turns, Now: now, Loc: loc, Errors: errs,
	})
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func chainAgent(t *testing.T, snap publicprofile.Snapshot, id string) publicprofile.Breakdown {
	t.Helper()
	for _, row := range snap.Periods.All.ByAgent {
		if row.ID == id {
			return row
		}
	}
	t.Fatalf("missing %s in %+v", id, snap.Periods.All.ByAgent)
	return publicprofile.Breakdown{}
}

func assertRetainedCursor(t *testing.T, snap publicprofile.Snapshot, row publicprofile.Breakdown) {
	t.Helper()
	if row.Totals.Total.Value == nil || *row.Totals.Total.Value != chainObserved {
		t.Fatalf("tokens=%+v", row.Totals.Total)
	}
	if row.Coverage.Reason != "" || row.Coverage.Tokens == publicprofile.StatusUnavailable || row.Coverage.TokenSource != publicprofile.TokenSourceAccountAPI {
		t.Fatalf("coverage=%+v", row.Coverage)
	}
	win := row.Coverage.TokenWindow
	if win == nil || win.Label != "account usage" || win.To == "" || !(win.To < snap.AsOfDate) {
		t.Fatalf("window=%+v as_of=%s", win, snap.AsOfDate)
	}
}

func decideChain(local publicprofile.Snapshot, remote *publicprofile.Snapshot, total *int64, verifiedDate string, publishedAt, now time.Time) publicprofile.Decision {
	in := publicprofile.RefreshInput{
		Local:     local,
		Remote:    remote,
		Now:       now,
		WallKnown: true,
		Verified: publicprofile.VerifiedWall{
			Total:       total,
			AsOfDate:    verifiedDate,
			PublishedAt: publishedAt,
		},
	}
	if remote != nil && remote.SnapshotID != "" {
		in.Verified.Proven = true
		in.Verified.SnapshotID = remote.SnapshotID
		in.RemoteUpdatedAt = publishedAt
	}
	return publicprofile.DecideRefresh(in)
}

func chainReport(t *testing.T, events []event.UsageEvent, turns []event.TurnEvent, errs []string, now time.Time, loc *time.Location) report.Snapshot {
	t.Helper()
	snap, err := report.Build(events, turns, errs, report.Filter{}, now, loc)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func chainJSON(t *testing.T, snap report.Snapshot) string {
	t.Helper()
	var b strings.Builder
	if err := report.WriteJSON(&b, snap); err != nil {
		t.Fatal(err)
	}
	return b.String()
}

func assertNoProviderAPI(t *testing.T, events []event.UsageEvent) {
	t.Helper()
	for _, e := range events {
		if e.Source == "cursor" && e.Derivation == event.DeriveProviderAPI {
			t.Fatal("sealed cursor emitted a fresh account row")
		}
	}
}

func tokenSum(events []event.UsageEvent, source string) int64 {
	var n int64
	for _, e := range events {
		if source != "" && e.Source != source {
			continue
		}
		n += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	return n
}

func writeChainCursorDB(t *testing.T, home string, localTokens bool) {
	t.Helper()
	dir := filepath.Join(home, "Library", "Application Support", "Cursor", "User", "globalStorage")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(dir, "state.vscdb")
	db, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if _, err := db.Exec(`CREATE TABLE cursorDiskKV (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	if _, err := db.Exec(`CREATE TABLE ItemTable (key TEXT UNIQUE ON CONFLICT REPLACE, value BLOB)`); err != nil {
		t.Fatal(err)
	}
	request := `{"type":2,"createdAt":"2026-10-31T02:00:00.000Z"}`
	if localTokens {
		request = `{"type":2,"createdAt":"2026-10-31T02:00:00.000Z","tokenCount":{"inputTokens":100,"outputTokens":10}}`
	}
	rows := []struct{ key, value string }{
		{"bubbleId:sess-a:u1", `{"type":1,"createdAt":"2026-10-31T01:00:00.000Z"}`},
		{"bubbleId:sess-a:a1", request},
	}
	for _, row := range rows {
		if _, err := db.Exec(`INSERT INTO cursorDiskKV (key, value) VALUES (?, ?)`, row.key, row.value); err != nil {
			t.Fatal(err)
		}
	}
	const sealed = "cursor-safe-storage:v1:QUJDRA"
	for _, key := range []string{"cursorAuth/accessToken", "cursorAuth/refreshToken"} {
		if _, err := db.Exec(`INSERT INTO ItemTable (key, value) VALUES (?, ?)`, key, sealed); err != nil {
			t.Fatal(err)
		}
	}
}

func writeChainGrok(t *testing.T, home string, input int64) {
	t.Helper()
	dir := filepath.Join(home, ".grok", "sessions", "s1")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := map[string]any{
		"timestamp": time.Date(2026, 10, 31, 4, 0, 0, 0, time.UTC).UnixMilli(),
		"params": map[string]any{
			"sessionId": "s1",
			"update": map[string]any{
				"sessionUpdate": "turn_completed",
				"prompt_id":     "p1",
				"usage":         map[string]any{"inputTokens": input},
			},
		},
	}
	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "updates.jsonl"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func writeChainCodex(t *testing.T, home string, output int64) {
	t.Helper()
	dir := filepath.Join(home, ".codex", "sessions", "2026", "10", "31")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatal(err)
	}
	line := map[string]any{
		"timestamp": "2026-10-31T04:00:00Z",
		"type":      "event_msg",
		"payload": map[string]any{
			"type": "token_count",
			"info": map[string]any{
				"total_token_usage": map[string]any{"output_tokens": output},
			},
		},
	}
	raw, err := json.Marshal(line)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "rollout-chain.jsonl"), append(raw, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}
