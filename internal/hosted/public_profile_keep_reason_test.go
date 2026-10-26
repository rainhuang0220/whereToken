package hosted

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
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

func TestCorrectionHeaderPublishesTheExactDrop(t *testing.T) {
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
	lower := mustProjection(t, publicprofile.Input{
		Now: day.Add(time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 19_000)},
	})
	drop := *first.snap.Periods.All.Totals.Total.Value - *lower.snap.Periods.All.Totals.Total.Value
	puts := git.puts
	wrong := putProjectionHeaders(t, h, device, lower.raw, "evidence", "1")
	if wrong["kept"] != "previous" || wrong["reason"] != "regression" || git.puts != puts {
		t.Fatalf("wrong delta %+v puts %d", wrong, git.puts)
	}
	got := putProjectionHeaders(t, h, device, lower.raw, "evidence", strconv.FormatInt(drop, 10))
	if got["kept"] == "previous" || got["readme"] != "materialized" || git.puts == puts {
		st := readyStore(t)
		user, _ := st.UserByLogin(context.Background(), "rainhuang0220")
		pres, _ := st.Presentation(context.Background(), user.ID)
		row, _ := st.Projection(context.Background(), user.ID)
		t.Fatalf("evidenced %+v puts %d -> %d drop %d verified=%v %d asof=%s row=%d status=%s reason=%s", got, puts, git.puts, drop, pres.VerifiedSnapshotID, pres.VerifiedTotal.Int64, pres.VerifiedAsOf.String, row.TotalTokens, row.ReadmeStatus, row.PendingReason)
	}
	if publicSnapshotID(t, h, "rainhuang0220") != lower.snap.SnapshotID {
		t.Fatal("lower snapshot was not stored")
	}
	again := mustProjection(t, publicprofile.Input{
		Now: day.Add(2 * time.Minute), Loc: time.UTC, Version: "test",
		Events: []event.UsageEvent{authEvent("claude", "anthropic", day.Add(-time.Hour), 18_000)},
	})
	puts = git.puts
	kept := putProjection(t, h, device, again.raw)
	if kept["kept"] != "previous" || kept["reason"] != "regression" || git.puts != puts {
		t.Fatalf("second drop %+v", kept)
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
