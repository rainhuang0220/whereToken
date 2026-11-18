package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/event"
	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestPendingPublicationKeepsVerifiedPagesWithoutFailure(t *testing.T) {
	when := time.Date(2026, 11, 9, 10, 0, 0, 0, time.UTC)
	build := func(n int64) publicprofile.Snapshot {
		s, err := publicprofile.Build(publicprofile.Input{Now: when, Loc: time.UTC, Version: "fixture", IncludeModels: true, Events: []event.UsageEvent{{Source: "grok", Vendor: "xai", Model: "grok-4.6", Timestamp: when, Miss: n, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw}}})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	current, next := build(50), build(70)
	dir := t.TempDir()
	if err := publicprofile.WriteBundle(dir, current, publicprofile.DefaultPresentation()); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(filepath.Join(dir, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	env := map[string]any{"schema": "wheretoken.public-profile-live", "schema_version": 1, "data_revision": next.SnapshotID, "snapshot": next, "wall": map[string]any{"phase": "pending", "verified_snapshot_id": current.SnapshotID, "verified_total": 50, "verified_as_of_date": current.AsOfDate, "verified_at": "2026-11-09T10:00:00Z"}}
	b, _ := json.Marshal(env)
	wall := filepath.Join(t.TempDir(), "wall.json")
	if err := os.WriteFile(wall, b, 0600); err != nil {
		t.Fatal(err)
	}
	changed, err := sync(wall, dir)
	if err != nil || changed {
		t.Fatalf("normal pending state failed: changed=%v err=%v", changed, err)
	}
	after, err := os.ReadFile(filepath.Join(dir, "profile.json"))
	if err != nil || string(before) != string(after) {
		t.Fatal("pending data replaced verified Pages")
	}
}

func testWall(t *testing.T) []byte {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("..", "..", "public-profile", "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	var snap publicprofile.Snapshot
	if err = json.Unmarshal(b, &snap); err != nil {
		t.Fatal(err)
	}
	total := snap.Periods.All.Totals.Total.Value
	if total == nil {
		t.Fatal("fixture missing total")
	}
	env := map[string]any{
		"schema":         "wheretoken.public-profile-live",
		"schema_version": 1,
		"data_revision":  snap.SnapshotID,
		"snapshot":       snap,
		"presentation":   publicprofile.DefaultPresentation(),
		"wall": map[string]any{
			"phase": "verified", "verified_snapshot_id": snap.SnapshotID,
			"verified_as_of_date": snap.AsOfDate, "verified_total": *total,
			"verified_at": "2026-11-03T11:37:11Z",
		},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	return raw
}
func TestVerifiedFallbackRebuildKeepsWallIdentity(t *testing.T) {
	raw := testWall(t)
	files, changed, err := buildVerifiedFallback(raw, nil)
	if err != nil || !changed {
		t.Fatalf("build failed changed=%v err=%v", changed, err)
	}
	var snap publicprofile.Snapshot
	if err = json.Unmarshal(files["profile.json"], &snap); err != nil {
		t.Fatal(err)
	}
	var man struct {
		SnapshotID string `json:"snapshot_id"`
	}
	if err = json.Unmarshal(files["manifest.json"], &man); err != nil {
		t.Fatal(err)
	}
	if snap.SnapshotID != man.SnapshotID {
		t.Fatal("manifest drifted")
	}
	if !strings.Contains(string(files["preview-light.svg"]), snap.SnapshotID[7:20]) { // preview may store abbreviated ID
		if len(files["preview-light.svg"]) < 200 {
			t.Fatal("missing preview")
		}
	}
}
func TestUnverifiedAndRegressedWallCannotReplacePages(t *testing.T) {
	raw := testWall(t)
	var obj map[string]any
	if err := json.Unmarshal(raw, &obj); err != nil {
		t.Fatal(err)
	}
	for name, mutate := range map[string]func(){
		"pending":           func() { obj["wall"].(map[string]any)["phase"] = "pending" },
		"mismatched-total":  func() { obj["wall"].(map[string]any)["verified_total"] = float64(1) },
		"wrong-verified-id": func() { obj["wall"].(map[string]any)["verified_snapshot_id"] = "sha256:" + strings.Repeat("0", 64) },
	} {
		if err := json.Unmarshal(raw, &obj); err != nil {
			t.Fatal(err)
		}
		mutate()
		b, _ := json.Marshal(obj)
		if _, _, err := buildVerifiedFallback(b, nil); err == nil {
			t.Fatalf("%s unexpectedly accepted", name)
		}
	}
}

func TestReuploadOfSameVerifiedSnapshotDoesNotRewriteCommittedBundle(t *testing.T) {
	dir := t.TempDir()
	wall := filepath.Join(t.TempDir(), "wall.json")
	raw := testWall(t)
	if err := os.WriteFile(wall, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if changed, err := sync(wall, dir); err != nil || !changed {
		t.Fatalf("first mirror: changed=%v err=%v", changed, err)
	}
	var env map[string]any
	if err := json.Unmarshal(raw, &env); err != nil {
		t.Fatal(err)
	}
	env["snapshot"].(map[string]any)["generated_at"] = "2026-11-16T02:32:00Z"
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(wall, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if changed, err := sync(wall, dir); err != nil || changed {
		t.Fatalf("reupload should leave static bytes unchanged: changed=%v err=%v", changed, err)
	}
}

func TestSameDayOlderVerifiedRevisionCannotReplacePages(t *testing.T) {
	when := time.Date(2026, 11, 16, 10, 0, 0, 0, time.UTC)
	build := func(at time.Time, model string) publicprofile.Snapshot {
		t.Helper()
		s, err := publicprofile.Build(publicprofile.Input{Now: at, Loc: time.UTC, Version: "fixture", IncludeModels: true, Events: []event.UsageEvent{
			{Source: "grok", Vendor: "xai", Model: model, Timestamp: at, Miss: 50, Quality: event.QualityAuthoritative, Derivation: event.DeriveRaw},
		}})
		if err != nil {
			t.Fatal(err)
		}
		return s
	}
	current := build(when, "grok-4.6")
	older := build(when.Add(-time.Hour), "grok-4.5")
	env := map[string]any{
		"schema": "wheretoken.public-profile-live", "schema_version": 1,
		"data_revision": older.SnapshotID, "snapshot": older,
		"wall": map[string]any{"phase": "verified", "verified_snapshot_id": older.SnapshotID,
			"verified_as_of_date": older.AsOfDate, "verified_total": 50, "verified_at": "2026-11-16T11:00:00Z"},
	}
	raw, err := json.Marshal(env)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := buildVerifiedFallback(raw, &current); err == nil {
		t.Fatal("same-day older revision was accepted")
	}
}
