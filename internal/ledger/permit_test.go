package ledger

import (
	"testing"

	"github.com/rainhuang0220/whereToken/internal/event"
)

func TestPermitTracksTheNetDropAndDoesNotDouble(t *testing.T) {
	lg := openTest(t)
	first := ev("codex", "s", "a", 10)
	second := ev("codex", "s", "b", 20)
	if err := lg.CommitUsage([]event.UsageEvent{first, second}); err != nil {
		t.Fatal(err)
	}
	if _, _, ok, err := lg.CorrectionPermit(); err != nil || ok {
		t.Fatalf("ordinary insert granted a permit ok=%v err=%v", ok, err)
	}
	lower := first
	lower.Miss = 1
	if err := lg.CommitUsage([]event.UsageEvent{lower}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Tokens != 32 {
		t.Fatalf("ordinary scan lowered the fact to %d", st.Tokens)
	}
	if _, _, ok, _ := lg.CorrectionPermit(); ok {
		t.Fatal("ordinary lower scan granted a permit")
	}

	fixed := first
	fixed.Miss = 4
	applied, err := lg.Correct(fixed, "counted the cached prefix twice", ParserVersion+1)
	if err != nil || !applied {
		t.Fatal(err, applied)
	}
	reason, delta, ok, err := lg.CorrectionPermit()
	if err != nil || !ok || reason != "counted the cached prefix twice" || delta != 6 {
		t.Fatalf("permit %q %d ok=%v err=%v", reason, delta, ok, err)
	}
	applied, err = lg.Correct(fixed, "counted the cached prefix twice", ParserVersion+2)
	if err != nil || applied {
		t.Fatalf("same payload applied=%v err=%v", applied, err)
	}
	_, delta, ok, _ = lg.CorrectionPermit()
	if !ok || delta != 6 {
		t.Fatalf("repeat changed the permit to %d ok=%v", delta, ok)
	}

	other := second
	other.Miss = 17
	applied, err = lg.Correct(other, "session was split", ParserVersion+1)
	if err != nil || !applied {
		t.Fatal(err, applied)
	}
	reason, delta, ok, _ = lg.CorrectionPermit()
	if !ok || delta != 9 || reason != "session was split" {
		t.Fatalf("net permit %q %d", reason, delta)
	}

	id, _ := Key(other)
	ok, err = lg.Revert(id, "session was split")
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	reason, delta, ok, _ = lg.CorrectionPermit()
	if !ok || delta != 6 || reason != "session was split" {
		t.Fatalf("revert left %q %d ok=%v", reason, delta, ok)
	}
	id, _ = Key(first)
	if ok, err = lg.Revert(id, "counted the cached prefix twice"); err != nil || !ok {
		t.Fatal(err, ok)
	}
	if _, _, ok, err = lg.CorrectionPermit(); err != nil || ok {
		t.Fatalf("permit survived a full revert ok=%v err=%v", ok, err)
	}
	st, _ = lg.Status()
	if st.Tokens != 32 || st.Revisions != 4 {
		t.Fatalf("audit %+v", st)
	}
}
