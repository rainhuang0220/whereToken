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
			name: "plus 4999 at one hour stays 1M",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+4_999))
				return in
			},
			action: ActionNoChange,
			code:   CodeSkippedDelta,
		},
		{
			name: "plus 10000 can publish without a one-hour cooldown",
			mutate: func(in PublicationInput) PublicationInput {
				in.Candidate = gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_000_000+10_000))
				in.Now = at.Add(time.Hour - time.Nanosecond)
				return in
			},
			action: ActionPublishNow,
			code:   CodePublished,
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
			code:   CodeBlockedRegression,
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

func TestDecidePublicationMidnightMergeRetainsOneIntent(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	offset := 480
	verified := wall("sha256:prev", "2026-09-25", 1_000_000, time.Date(2026, 9, 25, 18, 0, 0, 0, loc))
	cand := gateSnap("sha256:next", StatusAvailable, "2026-09-25", intPtr(1_010_000))
	cand.OwnerUTCOffsetMinutes = &offset
	near := time.Date(2026, 9, 25, 20, 0, 0, 0, loc)
	due := time.Date(2026, 9, 26, 0, 0, 0, 0, loc)
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: near})
	if got.Action != ActionWaitUntil || !got.DueAt.Equal(due) {
		t.Fatalf("hold until midnight: %+v", got)
	}
	pending := PendingIntent{SnapshotID: cand.SnapshotID, DueAt: got.DueAt, Reason: ReasonCooldown}
	again := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Pending: pending, Now: near.Add(time.Minute)})
	if again.Action != ActionWaitUntil || !again.DueAt.Equal(due) {
		t.Fatalf("coalesce %+v", again)
	}
	next := gateSnap("sha256:newday", StatusAvailable, "2026-09-26", intPtr(1_010_000))
	next.OwnerUTCOffsetMinutes = &offset
	published := DecidePublication(PublicationInput{Candidate: next, Verified: verified, Pending: pending, Now: due})
	if published.Action != ActionPublishNow || published.Reason != ReasonDate {
		t.Fatalf("midnight %+v", published)
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

func TestDecidePublicationDSTDoesNotImposeAnUnrelatedHour(t *testing.T) {
	loc, err := time.LoadLocation("America/New_York")
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 11, 1, 5, 30, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-11-01", 1_000_000, start)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-11-01", intPtr(1_010_000))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: start.Add(time.Hour - time.Nanosecond).In(loc)})
	if got.Action != ActionPublishNow || got.Reason != ReasonUsage {
		t.Fatalf("DST transition should not add hourly cooldown %+v", got)
	}
	if start.In(loc).Format("15:04") != "01:30" || start.Add(time.Hour).In(loc).Format("15:04") != "01:30" {
		t.Fatal("invalid DST fixture")
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

func TestVerifiedRawTokenThresholdAndMidnightWindow(t *testing.T) {
	loc := time.FixedZone("CST", 8*3600)
	offset := 480
	day := "2026-11-04"
	baseline := int64(19_097_123_975)
	at := time.Date(2026, 11, 4, 15, 0, 0, 0, loc)
	old := wall("sha256:old", day, baseline, at.Add(-2*time.Hour))
	check := func(name string, at time.Time, delta int64, action string, due time.Time) {
		t.Helper()
		candidate := gateSnap("sha256:"+name, StatusPartial, day, intPtr(baseline+delta))
		candidate.OwnerUTCOffsetMinutes = &offset
		got := DecidePublication(PublicationInput{Candidate: candidate, Verified: old, Now: at})
		if got.Action != action {
			t.Errorf("%s action=%s want=%s", name, got.Action, action)
		}
		if !due.IsZero() && !got.DueAt.Equal(due) {
			t.Errorf("%s due=%s want=%s", name, got.DueAt, due)
		}
	}
	check("sub_0_01B", at, 9_999_999, ActionNoChange, time.Time{})
	check("exact_0_01B", at, 10_000_000, ActionPublishNow, time.Time{})
	check("one_after", at, 10_000_001, ActionPublishNow, time.Time{})
	beforeMidnight := time.Date(2026, 11, 5, 0, 0, 0, 0, loc)
	check("at_19_59", time.Date(2026, 11, 4, 19, 59, 59, 0, loc), 10_000_000, ActionPublishNow, time.Time{})
	check("at_20_00", time.Date(2026, 11, 4, 20, 0, 0, 0, loc), 10_000_000, ActionWaitUntil, beforeMidnight)
	check("at_23_59", time.Date(2026, 11, 4, 23, 59, 59, 0, loc), 30_000_000, ActionWaitUntil, beforeMidnight)
	// UTC 16:00 is midnight in the owner's +08 zone. Server UTC does not
	// change the defer logic and new local calendar day wins.
	midnightNow := time.Date(2026, 11, 4, 16, 0, 0, 0, time.UTC)
	tomorrow := gateSnap("sha256:tomorrow", StatusPartial, "2026-11-05", intPtr(baseline))
	tomorrow.OwnerUTCOffsetMinutes = &offset
	got := DecidePublication(PublicationInput{Candidate: tomorrow, Verified: old, Now: midnightNow})
	if got.Action != ActionPublishNow || got.Reason != ReasonDate {
		t.Fatalf("owner-local midnight was missed: %+v", got)
	}
	// Crossing 1B switches from 0.01M to 0.01B, without a rounded-tick shortcut.
	first := wall("sha256:little", "2026-11-04", 999_999_990, at)
	cross := gateSnap("sha256:cross", StatusAvailable, day, intPtr(1_000_000_010))
	cross.OwnerUTCOffsetMinutes = &offset
	if got := DecidePublication(PublicationInput{Candidate: cross, Verified: first, Now: at}); got.Action != ActionNoChange {
		t.Fatalf("crossing 1B should require raw 10M: %+v", got)
	}
}
