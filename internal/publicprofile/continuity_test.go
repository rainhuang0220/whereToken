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
