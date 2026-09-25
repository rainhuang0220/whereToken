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
	view    RemoteView
	puts    int
	bodies  [][]byte
	putErr  error
	commit  bool
	status  int
	fetches int
}

func (f *fakePublisher) Fetch(context.Context) (RemoteView, error) {
	f.fetches++
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

func TestApplyRefreshDeltaWaitsForOneHour(t *testing.T) {
	start := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	remote := mustBuild(t, usageInput(start, time.UTC, 1_000))
	pub := &fakePublisher{view: RemoteView{Found: true, Snapshot: remote, UpdatedAt: start}}
	earlyLocal := mustBuild(t, usageInput(start.Add(time.Hour-time.Nanosecond), time.UTC, 1_000+10_000))
	early := ApplyRefresh(context.Background(), earlyLocal, start.Add(time.Hour-time.Nanosecond), false, pub)
	if pub.puts != 0 || early.Decision.Publish || early.Decision.Code != CodeSkippedCooldown {
		t.Fatalf("before 1h puts=%d %+v", pub.puts, early.Decision)
	}
	local := mustBuild(t, usageInput(start.Add(time.Hour), time.UTC, 1_000+10_000))
	got := ApplyRefresh(context.Background(), local, start.Add(time.Hour), false, pub)
	if pub.puts != 1 || !got.Decision.Publish || got.Decision.Code != CodePublished {
		t.Fatalf("at 1h puts=%d %+v", pub.puts, got.Decision)
	}
}

func TestApplyRefreshOnePutPerHour(t *testing.T) {
	start := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	pub := &fakePublisher{}
	first := mustBuild(t, usageInput(start, time.UTC, 100))
	if got := ApplyRefresh(context.Background(), first, start, false, pub); !got.Decision.Publish || pub.puts != 1 {
		t.Fatalf("first puts=%d %+v", pub.puts, got.Decision)
	}
	pub.view = RemoteView{Found: true, Snapshot: first, UpdatedAt: start}
	for _, step := range []struct {
		d    time.Duration
		miss int64
	}{
		{10 * time.Minute, 20_000},
		{20 * time.Minute, 40_000},
		{50 * time.Minute, 80_000},
	} {
		now := start.Add(step.d)
		local := mustBuild(t, usageInput(now, time.UTC, step.miss))
		got := ApplyRefresh(context.Background(), local, now, false, pub)
		if pub.puts != 1 || got.Decision.Publish {
			t.Fatalf("inside the hour puts=%d %+v", pub.puts, got.Decision)
		}
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
	earlyNow := time.Date(2026, 9, 25, 0, 5, 0, 0, loc)
	early := mustBuild(t, usageInput(earlyNow, loc, 42_000))
	if got := ApplyRefresh(context.Background(), early, earlyNow, false, pub); got.Decision.Publish || pub.puts != 0 {
		t.Fatalf("00:05 puts=%d %+v", pub.puts, got.Decision)
	}
	readyNow := time.Date(2026, 9, 25, 0, 10, 0, 0, loc)
	local := mustBuild(t, usageInput(readyNow, loc, 42_000))
	if local.AsOfDate != "2026-09-25" {
		t.Fatalf("local as_of %s", local.AsOfDate)
	}
	got := ApplyRefresh(context.Background(), local, readyNow, false, pub)
	if pub.puts != 1 || got.Decision.Code != CodeDateRollover {
		t.Fatalf("00:10 puts=%d %+v", pub.puts, got.Decision)
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
	if committed.view.Snapshot.SnapshotID != local.SnapshotID {
		t.Fatal("remote id did not advance on the committed put")
	}
	again := ApplyRefresh(context.Background(), local, now, false, committed)
	if committed.puts != 1 || again.Decision.Publish {
		t.Fatalf("second look puts=%d publish=%v", committed.puts, again.Decision.Publish)
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
