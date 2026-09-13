package publicprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestBundleManifestCarriesSnapshotAndProvenance(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	files, err := Bundle(snap)
	if err != nil {
		t.Fatal(err)
	}
	var manifest, profile map[string]any
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(files["profile.json"], &profile); err != nil {
		t.Fatal(err)
	}
	id, _ := profile["snapshot_id"].(string)
	if id == "" || manifest["snapshot_id"] != id {
		t.Fatalf("manifest=%v profile id=%q", manifest, id)
	}
	if manifest["provenance"] != "local_sanitized_snapshot" {
		t.Fatalf("manifest provenance=%v", manifest["provenance"])
	}
	if !strings.Contains(string(files["preview-light.svg"]), id) {
		t.Fatal("preview does not identify its snapshot")
	}
}

func TestCommittedSyntheticDemoIsVisiblyMarked(t *testing.T) {
	root := filepath.Join("..", "..", "docs", "media", "public-profile-demo")
	profile, err := os.ReadFile(filepath.Join(root, "profile.json"))
	if err != nil {
		t.Fatal(err)
	}
	preview, err := os.ReadFile(filepath.Join(root, "preview-light.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(profile), `"kind": "synthetic_demo"`) {
		t.Fatal("demo provenance missing")
	}
	if !strings.Contains(string(preview), "DEMO DATA") {
		t.Fatal("demo preview is not visibly marked")
	}
}
