package hosted

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestKeptPreviousNamesRegression(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	device, _, _ := seedOwner(t)
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	first := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 20_000)},
	})
	if got := putProjection(t, h, device, first.raw); got["readme"] != "materialized" {
		t.Fatalf("first %+v", got)
	}
	puts := git.puts
	lower := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 19_000)},
	})
	kept := putProjection(t, h, device, lower.raw)
	if kept["kept"] != "previous" || kept["reason"] != "regression" || git.puts != puts {
		t.Fatalf("kept %+v puts %d -> %d", kept, puts, git.puts)
	}
}

func TestDeviceHeadersCannotForgeACorrection(t *testing.T) {
	git := newMemGit(publishReadme)
	st := readyStore(t)
	opts := publishOptions(st, git)
	h := NewMux(opts)
	device, _, _ := seedOwner(t)
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	first := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 20_000)},
	})
	if got := putProjection(t, h, device, first.raw); got["readme"] != "materialized" {
		t.Fatalf("first %+v", got)
	}
	lower := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 19_000)},
	})
	drop := *first.snap.Periods.All.Totals.Total.Value - *lower.snap.Periods.All.Totals.Total.Value
	puts := git.puts
	exact := strconv.FormatInt(drop, 10)
	wrong := putProjectionHeaders(t, h, device, lower.raw, "evidence", "1")
	assertKeptRegression(t, wrong, puts, git.puts)
	for _, attempt := range []struct{ reason, delta string }{
		{"evidence", exact},
		{"evidence", exact},
		{"", exact},
		{"evidence\nline", exact},
	} {
		got := putProjectionHeaders(t, h, device, lower.raw, attempt.reason, attempt.delta)
		assertKeptRegression(t, got, puts, git.puts)
	}
	var wg sync.WaitGroup
	errs := make(chan string, 8)
	for i := 0; i < 8; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			body := append([]byte(nil), lower.raw...)
			req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(body))
			req.Header.Set("Authorization", "Bearer "+device)
			req.Header.Set("Wheretoken-Correction-Reason", "evidence")
			req.Header.Set("Wheretoken-Correction-Delta", exact)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != http.StatusOK {
				errs <- rec.Body.String()
				return
			}
			var got map[string]any
			_ = json.Unmarshal(rec.Body.Bytes(), &got)
			if got["kept"] != "previous" || got["reason"] != "regression" {
				errs <- rec.Body.String()
			}
		}()
	}
	wg.Wait()
	close(errs)
	for msg := range errs {
		t.Error(msg)
	}
	if git.puts != puts {
		t.Fatalf("concurrent forge wrote github %d -> %d", puts, git.puts)
	}
	other := mustProjection(t, publicprofile.Input{
		Now: day.Add(2 * time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 18_000)},
	})
	otherDrop := *first.snap.Periods.All.Totals.Total.Value - *other.snap.Periods.All.Totals.Total.Value
	kept := putProjectionHeaders(t, h, device, other.raw, "other snapshot", strconv.FormatInt(otherDrop, 10))
	assertKeptRegression(t, kept, puts, git.puts)
	if publicSnapshotID(t, h, "rainhuang0220") != first.snap.SnapshotID {
		t.Fatal("forged drop replaced the public snapshot")
	}
	user, err := st.UserByLogin(context.Background(), "rainhuang0220")
	if err != nil {
		t.Fatal(err)
	}
	row, err := st.Projection(context.Background(), user.ID)
	if err != nil || row.SnapshotID != first.snap.SnapshotID || strings.HasPrefix(row.PendingReason, "corr:") {
		t.Fatalf("projection %+v %v", row, err)
	}
	s := &server{opts: opts, limiter: map[string][]time.Time{}}
	s.retryFailedReadmes(context.Background())
	if git.puts != puts || publicSnapshotID(t, h, "rainhuang0220") != first.snap.SnapshotID {
		t.Fatal("retry published a rejected drop")
	}
	pres, err := st.Presentation(context.Background(), user.ID)
	if err != nil || pres.ReadmeSnapshotID != first.snap.SnapshotID || !pres.VerifiedTotal.Valid || pres.VerifiedTotal.Int64 != *first.snap.Periods.All.Totals.Total.Value {
		t.Fatalf("verified moved %+v %v", pres, err)
	}
	higher := mustProjection(t, publicprofile.Input{
		Now: day.Add(3 * time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 40_000)},
	})
	up := putProjection(t, h, device, higher.raw)
	if up["kept"] == "previous" || up["snapshot_id"] != higher.snap.SnapshotID {
		t.Fatalf("upward %+v", up)
	}
	if up["readme"] != "pending" && up["readme"] != "materialized" {
		t.Fatalf("upward readme %+v", up)
	}
	if publicSnapshotID(t, h, "rainhuang0220") != higher.snap.SnapshotID {
		t.Fatal("upward snapshot was not stored")
	}
}

func TestCorrectionTagIsNotReplayedAfterACrash(t *testing.T) {
	git := newMemGit(publishReadme)
	st := readyStore(t)
	opts := publishOptions(st, git)
	h := NewMux(opts)
	device, _, _ := seedOwner(t)
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	first := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 20_000)},
	})
	if got := putProjection(t, h, device, first.raw); got["readme"] != "materialized" {
		t.Fatalf("first %+v", got)
	}
	lower := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 19_000)},
	})
	user, err := st.UserByLogin(context.Background(), "rainhuang0220")
	if err != nil {
		t.Fatal(err)
	}
	if err := st.SaveProjection(context.Background(), user.ID, lower.raw, lower.snap); err != nil {
		t.Fatal(err)
	}
	drop := *first.snap.Periods.All.Totals.Total.Value - *lower.snap.Periods.All.Totals.Total.Value
	if err := st.SetPublicationIntent(context.Background(), user.ID, lower.snap.SnapshotID, "failed", "corr:"+strconv.FormatInt(drop, 10), "github_write", sql.NullTime{}, 1); err != nil {
		t.Fatal(err)
	}
	puts := git.puts
	s := &server{opts: opts, limiter: map[string][]time.Time{}}
	s.retryFailedReadmes(context.Background())
	if git.puts != puts {
		t.Fatalf("stored corr tag published %d -> %d", puts, git.puts)
	}
	pres, err := st.Presentation(context.Background(), user.ID)
	if err != nil || pres.ReadmeSnapshotID != first.snap.SnapshotID || !pres.VerifiedTotal.Valid || pres.VerifiedTotal.Int64 != *first.snap.Periods.All.Totals.Total.Value {
		t.Fatalf("verified moved %+v %v", pres, err)
	}
	row, err := st.Projection(context.Background(), user.ID)
	if err != nil || strings.HasPrefix(row.PendingReason, "corr:") || row.ReadmeStatus == "applied" {
		t.Fatalf("tag still authorizes %+v %v", row, err)
	}
}

func TestOtherAccountCannotDropTheOwnerWall(t *testing.T) {
	git := newMemGit(publishReadme)
	h := publishMux(t, git)
	device, _, _ := seedOwner(t)
	const otherLogin = "wt-correction-other"
	t.Cleanup(func() { clearOwnerPublication(t, otherLogin) })
	day := time.Date(2026, 10, 20, 12, 0, 0, 0, time.UTC)
	owner := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 20_000)},
	})
	if got := putProjection(t, h, device, owner.raw); got["readme"] != "materialized" {
		t.Fatalf("owner %+v", got)
	}
	other := seedLogin(t, 9904242, otherLogin)
	clearOwnerPublication(t, otherLogin)
	otherHigh := mustProjection(t, publicprofile.Input{
		Now: day, Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 30_000)},
	})
	if got := putProjection(t, h, other, otherHigh.raw); got["snapshot_id"] != otherHigh.snap.SnapshotID || got["kept"] == "previous" {
		t.Fatalf("other first %+v", got)
	}
	otherLow := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 10_000)},
	})
	drop := *otherHigh.snap.Periods.All.Totals.Total.Value - *otherLow.snap.Periods.All.Totals.Total.Value
	kept := putProjectionHeaders(t, h, other, otherLow.raw, "cross account", strconv.FormatInt(drop, 10))
	if kept["kept"] != "previous" || kept["reason"] != "regression" {
		t.Fatalf("other drop %+v", kept)
	}
	if publicSnapshotID(t, h, "rainhuang0220") != owner.snap.SnapshotID {
		t.Fatal("other account changed the owner wall")
	}
	if publicSnapshotID(t, h, otherLogin) != otherHigh.snap.SnapshotID {
		t.Fatal("other account reused a header to lower its own wall")
	}
	forgedOwner := putProjectionHeaders(t, h, other, owner.raw, "steal", "1")
	if publicSnapshotID(t, h, "rainhuang0220") != owner.snap.SnapshotID {
		t.Fatalf("forged owner body %+v", forgedOwner)
	}
}

func assertKeptRegression(t *testing.T, got map[string]any, before, after int) {
	t.Helper()
	if got["kept"] != "previous" || got["reason"] != "regression" || before != after {
		t.Fatalf("kept %+v puts %d -> %d", got, before, after)
	}
}

func putProjectionHeaders(t *testing.T, h http.Handler, token string, raw []byte, reason, delta string) map[string]any {
	t.Helper()
	req := httptest.NewRequest(http.MethodPut, "/api/v1/sync/public-profile", bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("Wheretoken-Correction-Reason", reason)
	req.Header.Set("Wheretoken-Correction-Delta", delta)
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("sync %d %s", rec.Code, rec.Body.String())
	}
	var body map[string]any
	_ = json.Unmarshal(rec.Body.Bytes(), &body)
	return body
}
