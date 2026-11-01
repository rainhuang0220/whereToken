//go:build darwin

package cursor

import (
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
)

// TestLiveNoUIFilteredWindow reads this machine's Cursor session only when
// WHERETOKEN_CURSOR_LIVE=1. The keychain call sets the no-UI flag, so a
// missing allowance fails instead of opening a dialog. The ledger is a
// temp file. Output is counts, dates, and a subject hash.
func TestLiveNoUIFilteredWindow(t *testing.T) {
	if os.Getenv("WHERETOKEN_CURSOR_LIVE") != "1" {
		t.Skip("set WHERETOKEN_CURSOR_LIVE=1 for a no-UI read on this machine")
	}
	dir, err := os.UserHomeDir()
	if err != nil {
		t.Fatal("home unavailable")
	}
	home := testhome.New(dir)
	kind := StoredSessionKind(home)
	t.Logf("session=%s", kind)
	roots := Adapter{}.Discover(home)
	if len(roots) == 0 {
		t.Fatal("no cursor database")
	}
	path := resolveDB(roots[0].Path)
	access, _, err := authTokens(path, false)
	if err != nil || access == "" {
		t.Fatal("session unreadable")
	}
	plain := access
	if _, ok := sealedBlob(access); ok {
		password, err := cursorSafeStoragePassword(false)
		if err != nil {
			t.Fatalf("no-ui keychain: %s", err.Error())
		}
		plain, err = decryptChromeCBC(sealedBlobMust(access), deriveChromeKey(password))
		password = ""
		if err != nil || !usableSessionToken(plain) {
			t.Fatal("decrypted value is not a usable session")
		}
	} else if !usableSessionToken(plain) {
		t.Fatal("stored session is not usable")
	}
	sum := sha256.Sum256([]byte(JWTSubject(plain)))
	t.Logf("subject_sha256_8=%s", hex.EncodeToString(sum[:4]))

	now := time.Now().UTC()
	probe := Adapter{Probe: true, Now: func() time.Time { return now }}
	evs, err := probe.fetchUsageWithToken("live-probe", plain)
	if err != nil {
		t.Fatalf("probe window: %s", err.Error())
	}
	logUsage(t, "probe", evs, TakeAccountCoverage("live-probe"))

	full := Adapter{Now: func() time.Time { return now }}
	all, err := full.fetchUsageWithToken("live-full", plain)
	plain = ""
	if err != nil {
		t.Fatalf("full windows: %s", err.Error())
	}
	wins := TakeAccountCoverage("live-full")
	logUsage(t, "full", all, wins)

	lg, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.v1.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	old := []event.UsageEvent{{
		Source: "cursor", RequestID: "retained:2026-10-23",
		Timestamp:   time.Date(2026, 10, 23, 12, 0, 0, 0, time.UTC),
		Miss:        8_700_453_509,
		Quality:     event.QualityDegraded,
		Derivation:  event.DeriveRetained,
		SkipRequest: true,
	}}
	if err := lg.SaveObservation("cursor", "verified_snapshot", "retained", time.Date(2026, 10, 23, 17, 13, 41, 0, time.UTC), old); err != nil {
		t.Fatal(err)
	}
	for _, w := range wins {
		if err := lg.NoteCoveredWindow("cursor", w.From, w.To, windowEvents(all, w)); err != nil {
			t.Fatal(err)
		}
	}
	ranges := make([]ledger.DayRange, 0, len(wins))
	for _, w := range wins {
		ranges = append(ranges, ledger.DayRange{From: w.From, To: w.To})
	}
	extra, err := lg.DaysOutside("cursor", ranges, all)
	if err != nil {
		t.Fatal(err)
	}
	var kept int64
	for _, e := range extra {
		if e.Timestamp.UTC().Format("2006-01-02") == "2026-10-23" {
			kept += e.Miss
		}
	}
	chained, ok, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || !ok {
		t.Fatalf("chain ok=%v err=%v", ok, err)
	}
	t.Logf("isolated_ledger days=%d outside_2026_10_23=%d chain=%d", len(chained), kept, tokenTotal(chained))
}

func sealedBlobMust(value string) []byte {
	blob, ok := sealedBlob(value)
	if !ok {
		return nil
	}
	return blob
}

func windowEvents(events []event.UsageEvent, w CoverageWindow) []event.UsageEvent {
	var out []event.UsageEvent
	for _, e := range events {
		if e.Derivation != event.DeriveProviderAPI || e.Timestamp.IsZero() {
			continue
		}
		day := e.Timestamp.UTC().Format("2006-01-02")
		if day >= w.From && day <= w.To {
			out = append(out, e)
		}
	}
	return out
}

func logUsage(t *testing.T, label string, events []event.UsageEvent, wins []CoverageWindow) {
	t.Helper()
	var miss, read, write, output int64
	var n int
	var from, to string
	for _, e := range events {
		if e.Derivation != event.DeriveProviderAPI || e.RequestID == "cursor-api:measured-zero" {
			continue
		}
		n++
		miss += e.Miss
		read += e.CacheRead
		write += e.CacheCreate
		output += e.Output
		day := e.Timestamp.UTC().Format("2006-01-02")
		if day == "" {
			continue
		}
		if from == "" || day < from {
			from = day
		}
		if to == "" || day > to {
			to = day
		}
	}
	t.Logf("%s events=%d days=%s..%s input=%d cache_read=%d cache_write=%d output=%d total=%d windows=%d",
		label, n, from, to, miss, read, write, output, miss+read+write+output, len(wins))
	for _, w := range wins {
		t.Logf("%s window %s..%s", label, w.From, w.To)
	}
}

func tokenTotal(events []event.UsageEvent) int64 {
	var n int64
	for _, e := range events {
		n += e.Miss + e.CacheRead + e.CacheCreate + e.Output
	}
	return n
}
