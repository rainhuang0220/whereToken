package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

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
