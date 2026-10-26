package publicprofile

import (
	"context"
	"testing"
	"time"
)

func TestEvidencedDropPublishesOnceAndAnOrdinaryDropDoesNot(t *testing.T) {
	at := time.Date(2026, 10, 24, 3, 0, 0, 0, time.UTC)
	verified := wall("sha256:prev", "2026-10-24", 1_000, at)
	cand := gateSnap("sha256:next", StatusAvailable, "2026-10-24", intPtr(990))
	blocked := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: at.Add(2 * time.Hour)})
	if blocked.Action != ActionBlocked || blocked.Code != CodeBlockedRegression {
		t.Fatalf("ordinary %+v", blocked)
	}
	wrong := blocked
	wrongIn := PublicationInput{Candidate: cand, Verified: verified, Now: at.Add(2 * time.Hour), Correction: Correction{Reason: "evidence", Delta: 9}}
	wrong = DecidePublication(wrongIn)
	if wrong.Code != CodeBlockedRegression {
		t.Fatalf("wrong delta %+v", wrong)
	}
	if CorrectionAllows(" ", "10", 1_000, 990) || CorrectionAllows("evidence", "10x", 1_000, 990) || CorrectionAllows("bad\nreason", "10", 1_000, 990) {
		t.Fatal("malformed permit was accepted")
	}
	if !CorrectionAllows("  evidence  ", " 10 ", 1_000, 990) {
		t.Fatal("trimmed permit was rejected")
	}
	earlier := gateSnap("sha256:old", StatusAvailable, "2026-10-23", intPtr(990))
	rewound := DecidePublication(PublicationInput{
		Candidate: earlier, Verified: verified, Now: at.Add(2 * time.Hour),
		Correction: Correction{Reason: "evidence", Delta: 10},
	})
	if rewound.Action != ActionNoChange || rewound.Reason != ReasonEarlierDate {
		t.Fatalf("earlier date %+v", rewound)
	}
	allowed := DecidePublication(PublicationInput{
		Candidate: cand, Verified: verified, Now: at.Add(2 * time.Hour),
		Correction: Correction{Reason: "evidence", Delta: 10},
	})
	if allowed.Action != ActionPublishNow || allowed.Reason != ReasonCorrection || allowed.Code != CodePublished {
		t.Fatalf("evidenced %+v", allowed)
	}
	again := DecidePublication(PublicationInput{Candidate: cand, Verified: verified, Now: at.Add(2 * time.Hour)})
	if again.Code != CodeBlockedRegression {
		t.Fatalf("second apply %+v", again)
	}
	ctx := ContextWithCorrection(context.Background(), Correction{Reason: "evidence", Delta: 10})
	if got := CorrectionFrom(ctx); !got.Explains(1_000, 990) {
		t.Fatalf("context %+v", got)
	}
	if got := CorrectionFrom(ContextWithCorrection(ctx, Correction{})); got.Delta != 10 {
		t.Fatal("empty permit replaced a real one")
	}
}
