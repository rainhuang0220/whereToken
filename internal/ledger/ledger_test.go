package ledger

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/index"
)

func openTest(t *testing.T) *Ledger {
	t.Helper()
	lg, err := Open(filepath.Join(t.TempDir(), "ledger.v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { lg.Close() })
	return lg
}

func at(day int) time.Time {
	return time.Date(2026, 10, day, 1, 2, 3, 0, time.UTC)
}

func ev(source, session, req string, miss int64) event.UsageEvent {
	return event.UsageEvent{
		Source: source, SessionID: session, RequestID: req,
		Model: "m", Vendor: "v", Timestamp: at(23),
		Miss: miss, Output: 1,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw,
	}
}

func tokensOf(events []event.UsageEvent) int64 {
	var n int64
	for _, e := range events {
		n += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	return n
}

func TestPathIsOutsideTheScanCache(t *testing.T) {
	h := testhome.New(t.TempDir())
	lp, err := PathFor(h)
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Dir(lp) == filepath.Dir(index.PathFor(h)) {
		t.Fatalf("ledger %s shares the cache directory", lp)
	}
	if filepath.Base(lp) != "ledger.v1.db" {
		t.Fatal(lp)
	}
}

func TestKeyIgnoresPathAndKeepsDistinctRequests(t *testing.T) {
	a, ok := Key(ev("codex", "same", "/tmp/sessions/same.jsonl:7", 10))
	b, ok2 := Key(ev("codex", "same", "/tmp/archived_sessions/same.jsonl:7", 10))
	c, ok3 := Key(ev("codex", "other", "/tmp/sessions/other.jsonl:7", 10))
	d, ok4 := Key(ev("codex", "same", "req-stable", 10))
	if !ok || !ok2 || !ok3 || !ok4 || a != b || a == c || a == d {
		t.Fatalf("keys %q %q %q %q", a, b, c, d)
	}
	kimi, ok := Key(ev("kimi", "sess", "/tmp/day/file.jsonl:169:3:100", 1))
	if !ok || kimi != "kimi\x1fsess\x1f169:3:100" {
		t.Fatal(kimi)
	}
	if _, ok := Key(ev("codex", "", "/tmp/sessions/no-session.jsonl:1", 1)); ok {
		t.Fatal("path-scoped id without a session was accepted")
	}
	lg := openTest(t)
	left := ev("codex", "", "/tmp/a/one.jsonl:1", 10)
	right := ev("codex", "", "/tmp/b/two.jsonl:1", 20)
	if err := lg.CommitUsage([]event.UsageEvent{left, right}); err != nil {
		t.Fatal(err)
	}
	st, err := lg.Status()
	if err != nil || st.Facts != 0 {
		t.Fatalf("missing session collapsed into a fact %+v %v", st, err)
	}
	if _, ok := Key(event.UsageEvent{Source: "claude", Miss: 1}); ok {
		t.Fatal("empty request id was invented")
	}
	api := ev("cursor", "s", "cursor-api:s:1", 40)
	api.Derivation = event.DeriveProviderAPI
	api.SkipRequest = true
	if _, ok := Key(api); ok {
		t.Fatal("provider api became a fact key")
	}
}

func TestDeletedSessionsStayAndAreNotDoubleCounted(t *testing.T) {
	lg := openTest(t)
	all := make([]event.UsageEvent, 100)
	for i := range all {
		all[i] = ev("codex", fmt.Sprintf("s%03d", i), fmt.Sprintf("/var/sessions/s%03d.jsonl:1", i), 10)
	}
	if err := lg.CommitUsage(all); err != nil {
		t.Fatal(err)
	}
	if err := lg.CommitUsage(all); err != nil {
		t.Fatal(err)
	}
	kept := all[:70]
	extra, err := lg.Unseen(kept)
	if err != nil {
		t.Fatal(err)
	}
	if got := tokensOf(kept) + tokensOf(extra); got != tokensOf(all) {
		t.Fatalf("after drop 30: got %d want %d extra %d", got, tokensOf(all), len(extra))
	}
	st, err := lg.Status()
	if err != nil || st.Facts != 100 || st.Tokens != tokensOf(all) {
		t.Fatalf("status %+v err %v", st, err)
	}
	again, err := lg.Unseen(append(append([]event.UsageEvent{}, kept...), extra...))
	if err != nil || len(again) != 0 {
		t.Fatalf("replay doubled: %d %v", len(again), err)
	}
}

func TestSameSessionInTwoDirectoriesIsOneFact(t *testing.T) {
	lg := openTest(t)
	a := ev("codex", "same", "/tmp/sessions/same.jsonl:1", 10)
	b := ev("codex", "same", "/tmp/archived_sessions/same.jsonl:1", 4)
	if err := lg.CommitUsage([]event.UsageEvent{a, b}); err != nil {
		t.Fatal(err)
	}
	st, err := lg.Status()
	if err != nil || st.Facts != 1 || st.Tokens != 11 {
		t.Fatalf("collapsed wrong %+v %v", st, err)
	}
}

func TestEqualTokensWithDifferentIDsBothStay(t *testing.T) {
	lg := openTest(t)
	a := ev("codex", "a", "req-a", 10)
	b := ev("codex", "b", "req-b", 10)
	b.Timestamp = a.Timestamp
	if err := lg.CommitUsage([]event.UsageEvent{a, b}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Facts != 2 || st.Tokens != 22 {
		t.Fatalf("weak fingerprint collapsed %+v", st)
	}
}

func TestAppendAfterTruncateAddsOnlyTheNewSeq(t *testing.T) {
	lg := openTest(t)
	first := ev("codex", "s", "/tmp/sessions/s.jsonl:1", 10)
	second := ev("codex", "s", "/tmp/sessions/s.jsonl:2", 4)
	if err := lg.CommitUsage([]event.UsageEvent{first}); err != nil {
		t.Fatal(err)
	}
	if err := lg.CommitUsage([]event.UsageEvent{first, second}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Facts != 2 || st.Tokens != 16 {
		t.Fatalf("append %+v", st)
	}
}

func TestUnkeyedAndPartialBatch(t *testing.T) {
	lg := openTest(t)
	good := ev("claude", "s", "req-1", 5)
	bad := ev("claude", "s", "", 9)
	if err := lg.CommitUsage([]event.UsageEvent{good, bad}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Facts != 1 || st.Tokens != 6 {
		t.Fatalf("unkeyed stored %+v", st)
	}
}

func TestLowerReparseDoesNotOverwrite(t *testing.T) {
	lg := openTest(t)
	full := ev("claude", "s", "req", 10)
	full.Output = 5
	lower := full
	lower.Miss = 1
	lower.Output = 9
	if err := lg.CommitUsage([]event.UsageEvent{full}); err != nil {
		t.Fatal(err)
	}
	if err := lg.CommitUsage([]event.UsageEvent{lower}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Tokens != 15 {
		t.Fatalf("reparse overwrote %d", st.Tokens)
	}
	raised := full
	raised.Output = 8
	if err := lg.CommitUsage([]event.UsageEvent{raised}); err != nil {
		t.Fatal(err)
	}
	st, _ = lg.Status()
	if st.Tokens != 18 || st.Facts != 1 {
		t.Fatalf("superset %+v", st)
	}
}

func TestCorrectionIsAuditedOnceAndReversible(t *testing.T) {
	lg := openTest(t)
	orig := ev("codex", "s", "req", 10)
	if err := lg.CommitUsage([]event.UsageEvent{orig}); err != nil {
		t.Fatal(err)
	}
	fixed := orig
	fixed.Miss = 2
	applied, err := lg.Correct(fixed, "dedup-fix", ParserVersion)
	if applied || !errors.Is(err, ErrCorrectionRejected) {
		t.Fatalf("same version applied=%v err=%v", applied, err)
	}
	applied, err = lg.Correct(fixed, "dedup-fix", ParserVersion+1)
	if err != nil || !applied {
		t.Fatal(err, applied)
	}
	applied, err = lg.Correct(fixed, "dedup-fix", ParserVersion+1)
	if err != nil || applied {
		t.Fatalf("second correction applied=%v err=%v", applied, err)
	}
	st, _ := lg.Status()
	if st.Tokens != 3 || st.Revisions != 1 {
		t.Fatalf("after correct %+v", st)
	}
	id, _ := Key(orig)
	ok, err := lg.Revert(id, "dedup-fix")
	if err != nil || !ok {
		t.Fatal(err, ok)
	}
	ok, err = lg.Revert(id, "dedup-fix")
	if err != nil || ok {
		t.Fatalf("second revert ok=%v err=%v", ok, err)
	}
	st, _ = lg.Status()
	if st.Tokens != 11 {
		t.Fatalf("revert %+v", st)
	}
	if _, err := lg.Correct(orig, "", ParserVersion+2); !errors.Is(err, ErrCorrectionRejected) {
		t.Fatal(err)
	}
}

func TestImportIsIdempotentAndRollsBack(t *testing.T) {
	lg := openTest(t)
	batch := []event.UsageEvent{ev("codex", "s", "req", 10), ev("grok", "g", "req", 3)}
	lg.beforeImportCommit = func() error { return errors.New("power loss") }
	if err := lg.Import(batch, 2); err == nil {
		t.Fatal("expected rollback")
	}
	if lg.Imported() {
		t.Fatal("marked imported after rollback")
	}
	st, _ := lg.Status()
	if st.Facts != 0 {
		t.Fatalf("partial import %d", st.Facts)
	}
	lg.beforeImportCommit = nil
	if err := lg.Import(batch, 2); err != nil {
		t.Fatal(err)
	}
	bigger := ev("codex", "s", "req", 99)
	if err := lg.Import([]event.UsageEvent{bigger}, 0); err != nil {
		t.Fatal(err)
	}
	st, _ = lg.Status()
	if st.Facts != 2 || st.Tokens != 15 || st.ImportGaps != 2 || !st.Imported {
		t.Fatalf("import %+v", st)
	}
}

func TestConcurrentCommitDoesNotDouble(t *testing.T) {
	lg := openTest(t)
	var wg sync.WaitGroup
	errCh := make(chan error, 16)
	for g := 0; g < 8; g++ {
		wg.Add(1)
		go func(g int) {
			defer wg.Done()
			for n := 0; n < 20; n++ {
				e := ev("codex", fmt.Sprintf("g%d", g), fmt.Sprintf("req-%d", n), 1)
				if err := lg.CommitUsage([]event.UsageEvent{e}); err != nil {
					errCh <- err
					return
				}
				if err := lg.CommitUsage([]event.UsageEvent{e}); err != nil {
					errCh <- err
					return
				}
			}
		}(g)
	}
	wg.Wait()
	close(errCh)
	for err := range errCh {
		t.Fatal(err)
	}
	st, err := lg.Status()
	if err != nil || st.Facts != 160 || st.Tokens != 320 {
		t.Fatalf("concurrent %+v %v", st, err)
	}
}

func TestProviderAPIDoesNotAddToLocalFacts(t *testing.T) {
	lg := openTest(t)
	local := ev("cursor", "s", "bubble-1", 100)
	local.Output = 0
	if err := lg.CommitUsage([]event.UsageEvent{local}); err != nil {
		t.Fatal(err)
	}
	api := event.UsageEvent{
		Source: "cursor", SessionID: "s", RequestID: "cursor-api:s:1:m:40:0:0:0:0",
		Model: "m", Timestamp: at(23), Miss: 40,
		Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI, SkipRequest: true,
	}
	if err := lg.NoteAPI([]event.UsageEvent{api}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteAPI([]event.UsageEvent{api}); err != nil {
		t.Fatal(err)
	}
	revised := api
	revised.Miss = 41
	revised.RequestID = "cursor-api:s:1:m:41:0:0:0:0"
	if err := lg.NoteAPI([]event.UsageEvent{revised}); err != nil {
		t.Fatal(err)
	}
	extra, err := lg.Unseen([]event.UsageEvent{api})
	if err != nil || len(extra) != 0 {
		t.Fatalf("api scan added ledger facts %d %v", len(extra), err)
	}
	st, _ := lg.Status()
	if st.Facts != 1 || st.Tokens != 100 {
		t.Fatalf("api stored as fact %+v", st)
	}
	var n int
	if err := lg.db.QueryRow(`SELECT COUNT(*) FROM api_observations`).Scan(&n); err != nil || n != 2 {
		t.Fatalf("observations %d %v", n, err)
	}
}

func TestRereadOmitsReplacedRequestsAndDeletionKeepsTheLastRead(t *testing.T) {
	lg := openTest(t)
	old := ev("claude", "sess", "old", 99)
	old.Output = 0
	neu := ev("claude", "sess", "new", 3)
	neu.Output = 0
	if err := lg.CommitUsage([]event.UsageEvent{old}); err != nil {
		t.Fatal(err)
	}
	if err := lg.NoteComplete([]event.UsageEvent{neu}); err != nil {
		t.Fatal(err)
	}
	if err := lg.CommitUsage([]event.UsageEvent{neu}); err != nil {
		t.Fatal(err)
	}
	extra, err := lg.Unseen(nil)
	if err != nil {
		t.Fatal(err)
	}
	if tokensOf(extra) != 3 || len(extra) != 1 {
		t.Fatalf("replaced request returned %d events tokens %d", len(extra), tokensOf(extra))
	}
	st, err := lg.Status()
	if err != nil || st.Tokens != 3 || st.Facts != 1 {
		t.Fatalf("active %+v %v", st, err)
	}
	again := ev("claude", "sess", "old", 99)
	again.Output = 0
	if err := lg.NoteComplete([]event.UsageEvent{again, neu}); err != nil {
		t.Fatal(err)
	}
	extra, err = lg.Unseen(nil)
	if err != nil || tokensOf(extra) != 102 || len(extra) != 2 {
		t.Fatalf("archive reread %+v err %v", extra, err)
	}
}

func TestReasoningIsNotInTheTokenSum(t *testing.T) {
	lg := openTest(t)
	e := ev("grok", "s", "req", 0)
	e.Output = 10
	e.Reasoning = 10
	if err := lg.CommitUsage([]event.UsageEvent{e}); err != nil {
		t.Fatal(err)
	}
	st, _ := lg.Status()
	if st.Tokens != 10 {
		t.Fatalf("reasoning double counted %d", st.Tokens)
	}
}

func TestBackupRestoreMatchesBreakdown(t *testing.T) {
	lg := openTest(t)
	a := ev("codex", "s", "req-a", 10)
	b := ev("grok", "g", "req-b", 4)
	b.Timestamp = at(24)
	if err := lg.CommitUsage([]event.UsageEvent{a, b}); err != nil {
		t.Fatal(err)
	}
	wantDigest, err := lg.Digest()
	if err != nil {
		t.Fatal(err)
	}
	want, err := lg.Breakdown()
	if err != nil {
		t.Fatal(err)
	}
	backup := filepath.Join(t.TempDir(), "ledger.bak")
	if err := lg.BackupTo(backup); err != nil {
		t.Fatal(err)
	}
	path := lg.Path()
	lg.Close()
	if err := os.WriteFile(path, []byte("not a database"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := Restore(backup, path); err != nil {
		t.Fatal(err)
	}
	lg, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	gotDigest, err := lg.Digest()
	if err != nil || gotDigest != wantDigest {
		t.Fatalf("digest %s %s %v", gotDigest, wantDigest, err)
	}
	got, err := lg.Breakdown()
	if err != nil || got.Facts != want.Facts || got.Tokens != want.Tokens || got.BySource["codex"] != want.BySource["codex"] || got.BySource["grok"] != want.BySource["grok"] || got.ByDay["2026-10-23"] != want.ByDay["2026-10-23"] || got.ByDay["2026-10-24"] != want.ByDay["2026-10-24"] {
		t.Fatalf("breakdown got %+v want %+v err %v", got, want, err)
	}
}

func TestLedgerDoesNotStoreTheSourcePath(t *testing.T) {
	lg := openTest(t)
	e := ev("codex", "sess", "/Users/SECRETPATH/rollout.jsonl:4", 3)
	if err := lg.CommitUsage([]event.UsageEvent{e}); err != nil {
		t.Fatal(err)
	}
	lg.Close()
	raw, err := os.ReadFile(lg.Path())
	if err != nil {
		t.Fatal(err)
	}
	wal, _ := os.ReadFile(lg.Path() + "-wal")
	blob := append(raw, wal...)
	if bytes.Contains(blob, []byte("SECRETPATH")) {
		t.Fatal("source path was stored in the ledger")
	}
}

func TestSchemaMismatchDoesNotDropFacts(t *testing.T) {
	lg := openTest(t)
	if err := lg.CommitUsage([]event.UsageEvent{ev("codex", "s", "req", 1)}); err != nil {
		t.Fatal(err)
	}
	if _, err := lg.db.Exec(`UPDATE meta SET value='99' WHERE key='schema'`); err != nil {
		t.Fatal(err)
	}
	path := lg.Path()
	lg.Close()
	if _, err := Open(path); err == nil {
		t.Fatal("mismatched schema opened")
	}
	dbRaw, err := os.ReadFile(path)
	if err != nil || !bytes.Contains(dbRaw, []byte("codex")) {
		t.Fatal("schema mismatch dropped the ledger")
	}
}
