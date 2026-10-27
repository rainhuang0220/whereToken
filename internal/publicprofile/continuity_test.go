package publicprofile

import (
	"testing"
	"time"
)

func TestDisplayChangedUsesTheVisibleHundredth(t *testing.T) {
	if DisplayChanged(1_000_000, 1_000_000+4_999) || !DisplayChanged(1_000_000, 1_000_000+5_000) {
		t.Fatal("1M hundredth")
	}
	if DisplayChanged(1_000, 1_004) || !DisplayChanged(1_000, 1_005) {
		t.Fatal("1K hundredth")
	}
	const base = int64(15_940_000_000)
	if DisplayChanged(base, base+4_999_999) || !DisplayChanged(base, base+5_000_000) {
		t.Fatal("15.94B hundredth")
	}
}

func TestNewDateWithLowerTotalStaysBlocked(t *testing.T) {
	at := time.Date(2026, 10, 20, 2, 38, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-10-20", 1_000_000, at)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-10-23", intPtr(1_000_000-1))
	got := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: at.Add(72 * time.Hour)})
	if got.Action != ActionBlocked || got.Reason != ReasonNegative || got.Code != CodeBlockedRegression {
		t.Fatalf("regression %+v", got)
	}
}

func TestFailedPendingRetriesWithoutADisplayStep(t *testing.T) {
	at := time.Date(2026, 10, 20, 2, 38, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-10-20", 1_000_000, at)
	cand := gateSnap("sha256:retry", StatusAvailable, "2026-10-20", intPtr(1_000_000+4_999))
	pending := PendingIntent{SnapshotID: cand.SnapshotID, Failed: true, DueAt: at.Add(time.Hour)}
	in := PublicationInput{Candidate: cand, Verified: verified, Pending: pending, Now: at.Add(2 * time.Hour)}
	in.FromWorker = true
	worker := DecidePublication(in)
	if worker.Action != ActionPublishNow || worker.Reason != ReasonRetry {
		t.Fatalf("worker %+v", worker)
	}
	in.FromWorker = false
	client := DecidePublication(in)
	if client.Action != ActionRetryPending || client.Code != CodePendingGitHub {
		t.Fatalf("client %+v", client)
	}
}

func TestEighteenBillionTickUsesTheDisplayedHundredth(t *testing.T) {
	const live = int64(18_189_589_196)
	if FormatCompact(live) != "18.19B" {
		t.Fatal(FormatCompact(live))
	}
	if DisplayChanged(live, 18_194_999_999) {
		t.Fatal("18.19B moved before the next hundredth")
	}
	const next = int64(18_195_000_000)
	if FormatCompact(next) != "18.2B" || !DisplayChanged(live, next) {
		t.Fatalf("18.2B %s changed %v", FormatCompact(next), DisplayChanged(live, next))
	}
	if FormatCompact(999_994_999) != "999.99M" || FormatCompact(999_995_000) != "1B" || !DisplayChanged(999_994_999, 999_995_000) {
		t.Fatalf("M to B %s %s", FormatCompact(999_994_999), FormatCompact(999_995_000))
	}
	at := time.Date(2026, 10, 27, 16, 4, 1, 0, time.UTC)
	verified := wall("sha256:prev", "2026-10-27", live, at)
	inside := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:tick", StatusAvailable, "2026-10-27", intPtr(next)),
		Verified:  verified,
		Now:       at.Add(time.Hour - time.Nanosecond),
	})
	if inside.Action != ActionWaitUntil || inside.Code != CodeWaitingCooldown {
		t.Fatalf("visible tick inside the hour %+v", inside)
	}
	atHour := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:tick", StatusAvailable, "2026-10-27", intPtr(next)),
		Verified:  verified,
		Now:       at.Add(time.Hour),
	})
	if atHour.Action != ActionPublishNow || atHour.Reason != ReasonUsage {
		t.Fatalf("visible tick at one hour %+v", atHour)
	}
	sub := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:sub", StatusAvailable, "2026-10-27", intPtr(live+1)),
		Verified:  verified,
		Now:       at.Add(2 * time.Hour),
	})
	if sub.Action != ActionNoChange || sub.Reason != ReasonDelta {
		t.Fatalf("sub tick after an hour %+v", sub)
	}
	rolled := DecidePublication(PublicationInput{
		Candidate: gateSnap("sha256:next-day", StatusAvailable, "2026-10-28", intPtr(live)),
		Verified:  verified,
		Now:       at.Add(time.Nanosecond),
	})
	if rolled.Action != ActionPublishNow || rolled.Reason != ReasonDate || rolled.Code != CodeDateRollover {
		t.Fatalf("later local date %+v", rolled)
	}
	// A failed publish of an accepted snapshot retries without the display
	// gate. A sub-tick same-day total still waits when that attempt succeeded.
	pending := PendingIntent{SnapshotID: "sha256:retry", Failed: true, DueAt: at}
	worker := DecidePublication(PublicationInput{
		Candidate:  gateSnap("sha256:retry", StatusAvailable, "2026-10-27", intPtr(live+1)),
		Verified:   verified,
		Pending:    pending,
		Now:        at.Add(2 * time.Hour),
		FromWorker: true,
	})
	if worker.Action != ActionPublishNow || worker.Reason != ReasonRetry {
		t.Fatalf("failed pending retry %+v", worker)
	}
}

func TestSameSnapshotDoesNotPublishAgain(t *testing.T) {
	at := time.Date(2026, 10, 20, 2, 38, 0, 0, time.UTC)
	cand := gateSnap("sha256:same", StatusAvailable, "2026-10-20", intPtr(1_000_000))
	got := DecidePublication(PublicationInput{
		Candidate: cand,
		Verified:  WallFromSnapshot(cand, at),
		Now:       at.Add(2 * time.Hour),
	})
	if got.Action != ActionNoChange || got.Reason != ReasonSame {
		t.Fatalf("duplicate %+v", got)
	}
}
