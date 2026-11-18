package cli

import (
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter"
	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/credstore"
	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/ledger"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
	"github.com/rainhuang0220/whereToken/internal/scan"
)

func TestRefreshIncidentPublishesRetainedDataAsIncomplete(t *testing.T) {
	loc := time.FixedZone("Asia/Shanghai", 8*3600)
	observed := time.Date(2026, 11, 9, 10, 58, 44, 0, time.UTC)
	c := event.UsageEvent{Source: "cursor", Timestamp: observed, Miss: 100_000, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI}
	k := event.UsageEvent{Source: "kimi", Timestamp: time.Date(2026, 11, 13, 2, 25, 51, 0, loc), Miss: 3_282_835, Quality: event.QualityAuthoritative}
	old, err := publicprofile.Build(publicprofile.Input{
		Events: []event.UsageEvent{c}, Now: time.Date(2026, 11, 12, 10, 30, 0, 0, loc), Loc: loc,
	})
	if err != nil {
		t.Fatal(err)
	}
	now := time.Date(2026, 11, 16, 10, 32, 0, 0, loc)
	a, out, errb := testApp([]string{"profile", "refresh", "--quiet"})
	home := testhome.New(t.TempDir())
	a.Home, a.Now, a.Loc = home, func() time.Time { return now }, loc
	a.Creds = &memCreds{m: map[string]string{credstore.KeyDeviceToken: "test-device", credstore.KeyLogin: "test-user"}}
	c.Derivation = event.DeriveRetained
	a.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Events: []event.UsageEvent{c, k}, Errors: []string{"cursor: OS_PERMISSION"}}
	}
	if err := publicprofile.SaveRefreshSwitch(publicprofile.RefreshConfigPath(home), true); err != nil {
		t.Fatal(err)
	}
	var uploaded publicprofile.Snapshot
	a.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if req.Method == http.MethodPut {
			raw, _ := io.ReadAll(req.Body)
			if err := json.Unmarshal(raw, &uploaded); err != nil {
				t.Fatal(err)
			}
			return jsonRes(200, map[string]any{"ok": true, "readme": "materialized", "snapshot_id": uploaded.SnapshotID})
		}
		if uploaded.SnapshotID != "" {
			return verifiedResponse(uploaded, now)
		}
		return jsonRes(200, map[string]any{"snapshot": old})
	}
	if code := a.Run(); code != ExitOK {
		t.Fatalf("refresh %d: %s %s", code, out, errb)
	}
	if uploaded.AsOfDate != "2026-11-16" || uploaded.DataStatus != publicprofile.StatusPartial {
		t.Fatalf("published date=%s status=%s", uploaded.AsOfDate, uploaded.DataStatus)
	}
	for i, day := range uploaded.Activity.Dates {
		if day == "2026-11-14" || day == "2026-11-15" {
			if got := uploaded.Activity.Series[0].States[i]; got != publicprofile.CellUnknown {
				t.Fatalf("%s = %s, want unknown", day, got)
			}
		}
	}
	a.Args = []string{"profile", "refresh", "status"}
	out.Reset()
	if code := a.Run(); code != ExitFail || !strings.Contains(out.String(), "collection_healthy=no") || !strings.Contains(out.String(), "publication_verified=yes") {
		t.Fatalf("status %d: %s", code, out)
	}
}

func TestAuditedWindowCannotEraseObservedDay(t *testing.T) {
	lg, err := ledger.Open(filepath.Join(t.TempDir(), "ledger.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer lg.Close()
	at := time.Date(2026, 11, 10, 0, 20, 0, 0, time.UTC)
	first := ledger.WindowAudit{From: "2026-11-09", To: "2026-11-09", Validation: "ok", Declared: 1, RawRows: 1, Deduped: 1, Days: 1, Miss: 10}
	ev := event.UsageEvent{Source: "cursor", Timestamp: at.Add(-time.Hour), Miss: 10, Quality: event.QualityAuthoritative, Derivation: event.DeriveProviderAPI}
	if err := lg.NoteAuditedWindow("cursor", first, at, []event.UsageEvent{ev}); err != nil {
		t.Fatal(err)
	}
	zero := ledger.WindowAudit{From: first.From, To: first.To, Validation: "ok"}
	if err := lg.NoteAuditedWindow("cursor", zero, at.Add(time.Hour), nil); !errors.Is(err, ledger.ErrObservationRegression) {
		t.Fatalf("replacement returned %v", err)
	}
	windows, err := lg.AuditedWindows("cursor", first.From)
	if err != nil || len(windows) != 1 || windows[0].Zero || !windows[0].ObservedAt.Equal(at) {
		t.Fatalf("coverage %+v: %v", windows, err)
	}
	events, _, err := lg.ChainedRetainedEvents("cursor")
	if err != nil || len(events) != 1 || events[0].Miss != 10 {
		t.Fatalf("retained %+v: %v", events, err)
	}
}

func TestSignedOutScheduledRefreshStillCollects(t *testing.T) {
	t.Setenv(profileRefreshChildEnv, "1")
	a, _, errb := testApp([]string{"profile", "refresh", "--quiet"})
	a.Home = testhome.New(t.TempDir())
	a.Creds = &memCreds{m: map[string]string{}}
	now := a.Now()
	a.Scan = func(adapter.Home) scan.Result {
		return scan.Result{Collection: &publicprofile.CollectionEvidence{Sources: []publicprofile.SourceCheck{{
			SourceCoverage: publicprofile.SourceCoverage{Source: "kimi", Status: publicprofile.SourceFresh},
			LastAttemptAt:  now, LastObservationAt: &now,
		}}}}
	}
	a.HTTPDo = func(*http.Request) (*http.Response, error) {
		t.Fatal("publication attempted without credentials")
		return nil, nil
	}
	if code := a.Run(); code != ExitOK {
		t.Fatalf("refresh %d: %s", code, errb)
	}
	st, err := publicprofile.LoadRefreshState(publicprofile.RefreshStatePath(a.Home))
	_, healthy := st.Collection.At(now)
	if err != nil || !healthy || st.Publication.Category != "AUTH" {
		t.Fatalf("state %+v err=%v", st, err)
	}
}

func TestPagesDoctorChecksThePublishedPreviewsAfterManifestMatch(t *testing.T) {
	a, _, _ := testApp(nil)
	digest := "sha256:" + strings.Repeat("a", 64)
	p := publicprofile.PublicationEvidence{PagesExpected: true, VerifiedSnapshotID: digest, VerifiedAsOfDate: "2026-11-16"}
	a.HTTPDo = func(req *http.Request) (*http.Response, error) {
		if strings.HasSuffix(req.URL.Path, ".json") {
			return jsonRes(http.StatusOK, map[string]string{"snapshot_id": digest, "as_of_date": p.VerifiedAsOfDate})
		}
		id := digest
		if strings.HasSuffix(req.URL.Path, "-dark.svg") {
			id = "sha256:" + strings.Repeat("b", 64)
		}
		return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader(
			`<svg><desc>Coding-agent token usage through 2026-11-16.</desc><metadata id="wheretoken-snapshot-id">` + id + `</metadata></svg>`))}, nil
	}
	if got := a.pagesMirrorStatus(p); got != "MISMATCH" {
		t.Fatalf("pages = %s", got)
	}
}

func verifiedResponse(snap publicprofile.Snapshot, now time.Time) (*http.Response, error) {
	return jsonRes(200, map[string]any{"snapshot": snap, "wall": map[string]any{
		"verified_snapshot_id": snap.SnapshotID, "verified_as_of_date": snap.AsOfDate,
		"verified_total": snap.Periods.All.Totals.Total.Value, "verified_at": now.UTC().Format(time.RFC3339),
	}})
}
