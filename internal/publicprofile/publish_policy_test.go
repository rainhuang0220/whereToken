package publicprofile

import (
	"testing"
	"time"
)

func wall(id, date string, total int64, at time.Time) VerifiedWall {
	return VerifiedWall{Proven: true, SnapshotID: id, Total: intPtr(total), AsOfDate: date, PublishedAt: at}
}

func TestDecidePublicationThresholds(t *testing.T) {
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-25", 1_000_000, at)
	base := PublicationInput{Verified: verified, Now: at.Add(time.Hour)}
	cases := []struct {
		name   string
		mutate func(in PublicationInput) PublicationInput
		action string
		code   string
	}{
		{
			name: "plus 9999 at one hour",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+9_999))
				return in
			},
			action: ActionNoChange,
			code:   CodeSkippedDelta,
		},
		{
			name: "plus 10000 one nanosecond early",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+10_000))
				in.Now = at.Add(time.Hour - time.Nanosecond)
				return in
			},
			action: ActionWaitUntil,
			code:   CodeWaitingCooldown,
		},
		{
			name: "plus 10000 at exactly one hour",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+10_000))
				in.Now = at.Add(time.Hour)
				return in
			},
			action: ActionPublishNow,
			code:   CodePublished,
		},
		{
			name: "no growth",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000))
				return in
			},
			action: ActionNoChange,
			code:   CodeSkippedDelta,
		},
		{
			name: "negative growth",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000-1))
				return in
			},
			action: ActionBlocked,
			code:   CodeSkippedDelta,
		},
		{
			name: "missing verified total is not zero",
			mutate: func(in PublicationInput) PublicationInput {
				in.Verified.Total = nil
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(50_000))
				return in
			},
			action: ActionBlocked,
			code:   CodeBlockedData,
		},
		{
			name: "missing candidate total",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", nil)
				return in
			},
			action: ActionBlocked,
			code:   CodeBlockedData,
		},
		{
			name: "weaker coverage",
			mutate: func(in PublicationInput) PublicationInput {
				prev := gateSnap("sha256:prev", StatusAvailable, "2026-09-25", intPtr(1_000_000))
				in.Accepted = &prev
				in.Candidate = gateSnap("sha256:next", StatusPartial, "2026-09-25", intPtr(1_000_000+20_000))
				return in
			},
			action: ActionBlocked,
			code:   CodeBlockedData,
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := DecidePublication(tc.mutate(base))
			if got.Action != tc.action || got.Code != tc.code {
				t.Fatalf("got %+v", got)
			}
		})
	}
}

func TestDecidePublicationCooldownRetainsOneIntent(t *testing.T) {
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-25", 1_000_000, at)
	early := time.Date(2026, 9, 25, 12, 20, 0, 0, time.UTC)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_010_000))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: early})
	if got.Action != ActionWaitUntil || !got.DueAt.Equal(at.Add(time.Hour)) {
		t.Fatalf("wait %+v", got)
	}
	pending := PendingIntent{SnapshotID: cand.SnapshotID, DueAt: got.DueAt, Reason: ReasonCooldown}
	again := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Pending: pending, Now: early.Add(time.Minute)})
	if again.Action != ActionWaitUntil || !again.DueAt.Equal(got.DueAt) {
		t.Fatalf("second look %+v", again)
	}
	ready := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Pending: pending, Now: got.DueAt})
	if ready.Action != ActionPublishNow || ready.Reason != ReasonUsage {
		t.Fatalf("due %+v", ready)
	}
}

func TestDecidePublicationMidnightBypassesCooldown(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	last := time.Date(2026, 9, 24, 23, 59, 0, 0, loc)
	verified := wall("sha256:prev", "2026-09-24", 42_000, last)
	midnight := time.Date(2026, 9, 25, 0, 0, 0, 0, loc)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(42_000))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: midnight})
	if got.Action != ActionPublishNow || got.Reason != ReasonDate || got.Code != CodeDateRollover {
		t.Fatalf("midnight %+v", got)
	}
	repeat := DecidePublication(PublicationInput{Candidate: cand, Verified: WallFromSnapshot(cand, midnight), Now: midnight.Add(time.Minute)})
	if repeat.Action != ActionNoChange {
		t.Fatalf("repeat midnight %+v", repeat)
	}
}

func TestDecidePublicationCatchUpPublishesLatestDateOnly(t *testing.T) {
	last := time.Date(2026, 9, 24, 23, 50, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-24", 42_000, last)
	morning := time.Date(2026, 9, 27, 8, 0, 0, 0, time.UTC)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-09-27", intPtr(42_000))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: morning})
	if got.Action != ActionPublishNow || got.Reason != ReasonDate {
		t.Fatalf("catch-up %+v", got)
	}
}

func TestDecidePublicationDoesNotPublishEarlierDate(t *testing.T) {
	at := time.Date(2026, 9, 26, 12, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-26", 1_000, at)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000+20_000))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: at.Add(2 * time.Hour)})
	if got.Action != ActionNoChange || got.Reason != ReasonEarlierDate {
		t.Fatalf("westward %+v", got)
	}
	invalid := gateSnap("sha256:bad", StatusAvailable, "09/27/2026", intPtr(1_000))
	if got := DecidePublication(PublicationInput{Candidate: invalid, Verified: verified, Now: at.Add(2 * time.Hour)}); got.Action == ActionPublishNow && got.Reason == ReasonDate {
		t.Fatalf("invalid date %+v", got)
	}
}

func TestDecidePublicationDSTCooldownIsAbsolute(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-11-01", 1_000_000, start)
	early := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_010_000)),
		Verified:  verified,
		Now:       start.Add(time.Hour - time.Nanosecond),
	})
	if early.Action != ActionWaitUntil {
		t.Fatalf("dst early %+v", early)
	}
	ready := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_010_000)),
		Verified:  verified,
		Now:       start.Add(time.Hour),
	})
	if ready.Action != ActionPublishNow {
		t.Fatalf("dst ready %+v", ready)
	}
	if start.In(loc).Format("15:04") != "01:30" || start.Add(time.Hour).In(loc).Format("15:04") != "01:30" {
		t.Fatal("fixture did not cross the repeated hour")
	}
}

func TestDecidePublicationThemeDoesNotWaitAndUsesLatest(t *testing.T) {
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-25", 1_000, at)
	latest := gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_500))
	got := DecidePublication(PublicationInput{Candidate: latest, Verified: verified, Now: at.Add(time.Minute), ThemeChange: true})
	if got.Action != ActionPublishNow || got.Reason != ReasonTheme {
		t.Fatalf("theme %+v", got)
	}
	earlier := gateSnap("sha256:old", StatusAvailable, "2026-09-24", intPtr(1_500))
	blocked := DecidePublication(PublicationInput{Candidate: earlier, Verified: verified, Now: at.Add(time.Minute), ThemeChange: true})
	if blocked.Action != ActionNoChange || blocked.Reason != ReasonEarlierDate {
		t.Fatalf("theme earlier %+v", blocked)
	}
}

func TestDecidePublicationFailedIntentCoalescesUntilWorker(t *testing.T) {
	at := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-09-25", 1_000, at.Add(-2*time.Hour))
	newest := gateSnap("sha256:newest", StatusAvailable, "2026-09-25", intPtr(20_000))
	pending := PendingIntent{SnapshotID: "sha256:failed", Failed: true, Reason: ReasonRetry}
	client := DecidePublication(PublicationInput{Candidate: newest, Verified: verified, Pending: pending, Now: at})
	if client.Action != ActionRetryPending {
		t.Fatalf("client %+v", client)
	}
	worker := DecidePublication(PublicationInput{Candidate: newest, Verified: verified, Pending: pending, Now: at, FromWorker: true})
	if worker.Action != ActionPublishNow || worker.Reason != ReasonRetry {
		t.Fatalf("worker %+v", worker)
	}
}

func TestDecidePublicationFirstSnapshotAndUnknownLegacy(t *testing.T) {
	now := time.Date(2026, 9, 25, 12, 0, 0, 0, time.UTC)
	first := DecidePublication(PublicationInput{Candidate: gateSnap("sha256:a", StatusAvailable, "2026-09-25", intPtr(1)), Now: now})
	if first.Action != ActionPublishNow || first.Reason != ReasonFirst {
		t.Fatalf("first %+v", first)
	}
	legacy := VerifiedWall{Proven: true, SnapshotID: "sha256:applied", PublishedAt: now.Add(-2 * time.Hour)}
	got := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:newer", StatusAvailable, "2026-09-26", intPtr(50_000)),
		Verified:  legacy,
		Now:       now,
	})
	if got.Action != ActionBlocked || got.Reason != ReasonUnknownVerified {
		t.Fatalf("legacy unknown %+v", got)
	}
}
