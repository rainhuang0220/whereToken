package publicprofile

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
)

type fakePublisher struct {
	view     RemoteView
	puts     int
	bodies   [][]byte
	putErr   error
	commit   bool
	status   int
	fetches  int
	fetchErr func(n int) error
	local    Snapshot
}

func (f *fakePublisher) Fetch(context.Context) (RemoteView, error) {
	f.fetches++
	if f.fetchErr != nil {
		if err := f.fetchErr(f.fetches); err != nil {
			return RemoteView{}, err
		}
	}
	if f.local.SnapshotID != "" && f.puts > 0 {
		return RemoteView{Found: true, Snapshot: f.local, UpdatedAt: f.view.UpdatedAt}, nil
	}
	return f.view, nil
}

func (f *fakePublisher) Put(_ context.Context, body []byte) (PutResult, error) {
	f.puts++
	f.bodies = append(f.bodies, append([]byte(nil), body...))
	if f.commit {
		var snap Snapshot
		if err := json.Unmarshal(body, &snap); err != nil {
			return PutResult{}, err
		}
		f.view = RemoteView{Found: true, Snapshot: snap, UpdatedAt: f.view.UpdatedAt}
	}
	if f.putErr != nil {
		return PutResult{}, f.putErr
	}
	status := f.status
	if status == 0 {
		status = 200
	}
	return PutResult{Status: status}, nil
}

func usageInput(now time.Time, loc *time.Location, miss int64) Input {
	if loc == nil {
		loc = time.UTC
	}
	return Input{
		Now: now, Loc: loc, Version: "test",
		Events: []event.UsageEvent{{
			Source: "claude", Vendor: "anthropic", Timestamp: now.Add(-time.Hour), Miss: miss, Quality: event.QualityAuthoritative,
		}},
	}
}

func mustBuild(t *testing.T, in Input) Snapshot {
	t.Helper()
	snap, err := Build(in)
	if err != nil {
		t.Fatal(err)
	}
	return snap
}

func TestApplyRefreshUnavailableDoesNotPut(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	local := mustBuild(t, Input{Now: now, Loc: time.UTC, Version: "test"})
	if local.DataStatus != StatusUnavailable {
		t.Fatalf("status %s", local.DataStatus)
	}
	pub := &fakePublisher{}
	got := ApplyRefresh(context.Background(), local, now, false, pub)
	if pub.puts != 0 {
		t.Fatalf("puts %d", pub.puts)
	}
	line := "phase=" + got.Decision.Phase + " " + got.Decision.Label
	if strings.Contains(line, "0.00 M") || strings.Contains(line, "$0") || strings.Contains(line, "#0") {
		t.Fatalf("zero display in %s", line)
	}
	if got.Decision.Publish {
		t.Fatal("unavailable published")
	}
}

func TestApplyRefreshSameDayDeltaWaitsForMidnight(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	start := time.Date(2026, 9, 25, 19, 0, 0, 0, loc)
	remote := mustBuild(t, usageInput(start, loc, 1000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: start}}
	before := time.Date(2026, 9, 25, 20, 0, 0, 0, loc)
	cand := mustBuild(t, usageInput(before, loc, 11000))
	first := ApplyRefresh(context.Background(), cand, before, false, pub)
	if pub.puts != 1 || first.Decision.Code != CodeWaitingCooldown {
		t.Fatalf("before midnight puts=%d %+v", pub.puts, first.Decision)
	}
	midnight := time.Date(2026, 9, 26, 0, 0, 0, 0, loc)
	if !first.Decision.DueAt.Equal(midnight) {
		t.Fatalf("due %v", first.Decision.DueAt)
	}
	pub.view = RemoteView{Found: true, Snapshot: cand, UpdatedAt: before, WallKnown: true, Wall: WallFromSnapshot(remote, start), Pending: PendingIntent{SnapshotID: cand.SnapshotID, DueAt: midnight}}
	again := ApplyRefresh(context.Background(), cand, before.Add(time.Minute), false, pub)
	if pub.puts != 1 || again.Decision.Publish {
		t.Fatalf("duplicate pending upload puts=%d %+v", pub.puts, again.Decision)
	}
	rolled := mustBuild(t, usageInput(midnight, loc, 11000))
	last := ApplyRefresh(context.Background(), rolled, midnight, false, pub)
	if pub.puts != 2 || last.Decision.Code != CodeDateRollover {
		t.Fatalf("midnight puts=%d %+v", pub.puts, last.Decision)
	}
}

func TestApplyRefreshQualifiedDeltaDoesNotWaitOneHour(t *testing.T) {
	start := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	pub := &fakePublisher{}
	first := mustBuild(t, usageInput(start, time.UTC, 100))
	if got := ApplyRefresh(context.Background(), first, start, false, pub); !got.Decision.Publish || pub.puts != 1 {
		t.Fatalf("first: %+v puts=%d", got, pub.puts)
	}
	pub.view = RemoteView{Found: true, Snapshot: first, UpdatedAt: start, WallKnown: true, Wall: WallFromSnapshot(first, start)}
	now := start.Add(10 * time.Minute)
	local := mustBuild(t, usageInput(now, time.UTC, 20000))
	got := ApplyRefresh(context.Background(), local, now, false, pub)
	if pub.puts != 2 || got.Decision.Code != CodePublished {
		t.Fatalf("qualified growth should publish immediately: %+v puts=%d", got, pub.puts)
	}
	pub.view = RemoteView{Found: true, Snapshot: local, UpdatedAt: now, WallKnown: true, Wall: WallFromSnapshot(local, now)}
	again := ApplyRefresh(context.Background(), local, now.Add(time.Minute), false, pub)
	if pub.puts != 2 || again.Decision.Publish {
		t.Fatalf("verified unchanged should not repeat PUT: %+v puts=%d", again, pub.puts)
	}
}

func TestApplyRefreshMidnightRolloverUpdatesAsOfDate(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	last := time.Date(2026, 9, 24, 23, 10, 0, 0, loc)
	remote := mustBuild(t, usageInput(last, loc, 42_000))
	if remote.AsOfDate != "2026-09-24" {
		t.Fatalf("remote as_of %s", remote.AsOfDate)
	}
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: last}}
	earlyNow := time.Date(2026, 9, 25, 0, 0, 0, 0, loc)
	early := mustBuild(t, usageInput(earlyNow, loc, 42_000))
	if early.AsOfDate != "2026-09-25" {
		t.Fatalf("local as_of %s", early.AsOfDate)
	}
	got := ApplyRefresh(context.Background(), early, earlyNow, false, pub)
	if pub.puts != 1 || got.Decision.Code != CodeDateRollover {
		t.Fatalf("00:00 puts=%d %+v", pub.puts, got.Decision)
	}
	var uploaded Snapshot
	if err := json.Unmarshal(pub.bodies[0], &uploaded); err != nil {
		t.Fatal(err)
	}
	if uploaded.AsOfDate != "2026-09-25" {
		t.Fatalf("uploaded as_of %s", uploaded.AsOfDate)
	}
	if uploaded.Provenance.RefreshMode != RefreshManualPublish || uploaded.Provenance.LiveSync {
		t.Fatalf("provenance %+v", uploaded.Provenance)
	}
}

func TestApplyRefreshTransportRetriesOnceAndDoesNotRepostAStoredSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1_000))
	local := mustBuild(t, usageInput(now, time.UTC, 1_000+10_000))
	boom := errors.New("transport")

	lost := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}, putErr: boom}
	first := ApplyRefresh(context.Background(), local, now, false, lost)
	if !first.Transport || lost.puts != 1 || lost.view.Snapshot.SnapshotID != remote.SnapshotID {
		t.Fatalf("lost puts=%d id=%s transport=%v", lost.puts, lost.view.Snapshot.SnapshotID, first.Transport)
	}
	second := ApplyRefresh(context.Background(), local, now, false, lost)
	if lost.puts != 2 || !second.Transport {
		t.Fatalf("retry puts=%d transport=%v", lost.puts, second.Transport)
	}

	committed := &fakePublisher{
		view:   RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)},
		putErr: boom,
		commit: true,
	}
	done := ApplyRefresh(context.Background(), local, now, false, committed)
	if done.Transport || !done.Already || committed.puts != 1 {
		t.Fatalf("committed puts=%d already=%v transport=%v", committed.puts, done.Already, done.Transport)
	}
	if done.Decision.Phase != PhaseWaiting || done.Decision.Code != CodePendingGitHub {
		t.Fatalf("stored projection is not proof of GitHub publication: %+v", done.Decision)
	}
	if committed.view.Snapshot.SnapshotID != local.SnapshotID {
		t.Fatal("remote id did not advance on the committed put")
	}
	again := ApplyRefresh(context.Background(), local, now, false, committed)
	if committed.puts != 1 || again.Decision.Publish {
		t.Fatalf("second look puts=%d publish=%v", committed.puts, again.Decision.Publish)
	}
}

func TestApplyRefreshDoesNotPutWhenRemoteAlreadyHasTheSnapshot(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	local := mustBuild(t, usageInput(now, time.UTC, 8_000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: local, UpdatedAt: now.Add(-2 * time.Hour)}}
	got := ApplyRefresh(context.Background(), local, now, false, pub)
	if pub.puts != 0 || got.Decision.Publish {
		t.Fatalf("puts=%d publish=%v %+v", pub.puts, got.Decision.Publish, got.Decision)
	}
}

func TestStoredUploadCanConfirmTheMatchingVerifiedWall(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1000))
	local := mustBuild(t, usageInput(now, time.UTC, 11000))
	p := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote}, putErr: errors.New("lost response"), commit: true}
	p.fetchErr = func(n int) error {
		if n == 2 {
			p.view.WallKnown = true
			p.view.Wall = WallFromSnapshot(p.view.Snapshot, now)
		}
		return nil
	}
	got := ApplyRefresh(context.Background(), local, now, false, p)
	if !got.Already || got.Transport || got.Decision.Code != CodeVerified || got.Decision.Phase != PhasePublished {
		t.Fatalf("verified wall ignored: %+v", got)
	}
}

func TestApplyRefreshRateLimitRetriesOnTheNextInvocation(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1_000))
	local := mustBuild(t, usageInput(now, time.UTC, 1_000+10_000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}, status: 429}
	first := ApplyRefresh(context.Background(), local, now, false, pub)
	if !first.Transport || first.Rejected || pub.puts != 1 {
		t.Fatalf("first puts=%d transport=%v rejected=%v", pub.puts, first.Transport, first.Rejected)
	}
	second := ApplyRefresh(context.Background(), local, now, false, pub)
	if pub.puts != 2 || !second.Transport {
		t.Fatalf("retry puts=%d transport=%v", pub.puts, second.Transport)
	}
}

func TestApplyRefreshConfirmFetchFailureThenLaterGetDoesNotPutAgain(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1_000))
	local := mustBuild(t, usageInput(now, time.UTC, 1_000+10_000))
	boom := errors.New("transport")
	pub := &fakePublisher{
		view:   RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)},
		putErr: boom,
		local:  local,
		fetchErr: func(n int) error {
			if n == 2 {
				return boom
			}
			return nil
		},
	}
	first := ApplyRefresh(context.Background(), local, now, false, pub)
	if !first.Transport || first.Already || pub.puts != 1 {
		t.Fatalf("confirm failed puts=%d transport=%v already=%v", pub.puts, first.Transport, first.Already)
	}
	pub.fetchErr = nil
	second := ApplyRefresh(context.Background(), local, now, false, pub)
	if pub.puts != 1 || second.Decision.Publish {
		t.Fatalf("later get put again puts=%d publish=%v", pub.puts, second.Decision.Publish)
	}
}

func TestApplyRefreshHTTP500RetriesAndHTTP400DoesNot(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1_000))
	local := mustBuild(t, usageInput(now, time.UTC, 1_000+10_000))
	serverErr := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}, status: 500}
	first := ApplyRefresh(context.Background(), local, now, false, serverErr)
	if !first.Transport || serverErr.puts != 1 {
		t.Fatalf("500 puts=%d transport=%v", serverErr.puts, first.Transport)
	}
	second := ApplyRefresh(context.Background(), local, now, false, serverErr)
	if serverErr.puts != 2 || !second.Transport {
		t.Fatalf("500 retry puts=%d transport=%v", serverErr.puts, second.Transport)
	}
	rejected := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}, status: 400}
	got := ApplyRefresh(context.Background(), local, now, false, rejected)
	if !got.Rejected || got.Transport || rejected.puts != 1 || rejected.fetches != 1 {
		t.Fatalf("400 puts=%d fetches=%d rejected=%v transport=%v", rejected.puts, rejected.fetches, got.Rejected, got.Transport)
	}
}

func TestApplyRefreshNilTotalsDoNotPublishOrPrintZero(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", nil)
	local := gateSnap("sha256:next", StatusAvailable, "2026-09-25", nil)
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}}
	got := ApplyRefresh(context.Background(), local, now, false, pub)
	line := got.Decision.Phase + " " + got.Decision.Label + " " + got.Decision.Code
	if pub.puts != 0 || got.Decision.Publish {
		t.Fatalf("nil totals published puts=%d %+v", pub.puts, got.Decision)
	}
	if strings.Contains(line, "0.00 M") || strings.Contains(line, "$0") || strings.Contains(line, "#0") {
		t.Fatalf("zero display %s", line)
	}
	drop := gateSnap("sha256:next", StatusAvailable, "2026-09-26", intPtr(999), measured("claude", 999))
	prev := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1_000), measured("claude", 1_000))
	pub = &fakePublisher{view: RemoteView{Found: true, Snapshot: prev, UpdatedAt: now.Add(-2 * time.Hour)}}
	got = ApplyRefresh(context.Background(), drop, now, false, pub)
	if pub.puts != 0 || got.Decision.Publish || got.Decision.Code != CodeBlockedRegression {
		t.Fatalf("drop on a new date puts=%d %+v", pub.puts, got.Decision)
	}
}

func TestApplyRefreshMorningCatchUpIsNotAMidnightPublish(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	last := time.Date(2026, 9, 24, 18, 0, 0, 0, loc)
	remote := mustBuild(t, usageInput(last, loc, 42_000))
	if remote.AsOfDate != "2026-09-24" {
		t.Fatalf("remote as_of %s", remote.AsOfDate)
	}
	morning := time.Date(2026, 9, 25, 8, 0, 0, 0, loc)
	if morning.Sub(last) < time.Hour {
		t.Fatalf("sub %s", morning.Sub(last))
	}
	local := mustBuild(t, usageInput(morning, loc, 42_000))
	if local.AsOfDate != "2026-09-25" {
		t.Fatalf("local as_of %s", local.AsOfDate)
	}
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: last}}
	got := ApplyRefresh(context.Background(), local, morning, false, pub)
	if pub.puts != 1 || got.Decision.Code != CodeDateRollover {
		t.Fatalf("08:00 puts=%d %+v", pub.puts, got.Decision)
	}
	if strings.Contains(got.Decision.Label, "00:00") {
		t.Fatalf("label claims midnight: %s", got.Decision.Label)
	}
}

func TestApplyRefreshSubMillionUsesOneTokenMinimum(t *testing.T) {
	start := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(start, time.UTC, 1_000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: start}}
	local := mustBuild(t, usageInput(start.Add(time.Hour), time.UTC, 1_004))
	got := ApplyRefresh(context.Background(), local, start.Add(time.Hour), false, pub)
	if pub.puts != 1 || !got.Decision.Publish || got.Decision.Code != CodePublished {
		t.Fatalf("sub-million uses one-token granularity puts=%d %+v", pub.puts, got.Decision)
	}
}

func TestApplyRefreshRejectsValidationWithoutAnotherPut(t *testing.T) {
	now := time.Date(2026, 9, 25, 15, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(now.Add(-2*time.Hour), time.UTC, 1_000))
	local := mustBuild(t, usageInput(now, time.UTC, 1_000+10_000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: now.Add(-2 * time.Hour)}, status: 400}
	got := ApplyRefresh(context.Background(), local, now, false, pub)
	if !got.Rejected || pub.puts != 1 || pub.fetches != 1 {
		t.Fatalf("puts=%d fetches=%d rejected=%v", pub.puts, pub.fetches, got.Rejected)
	}
}
