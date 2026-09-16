package publicprofile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/profilewebembed"
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
	assetRevision, _ := manifest["asset_revision"].(string)
	if assetRevision == "" || assetRevision == id {
		t.Fatalf("asset revision must exist independently of snapshot id: %q", assetRevision)
	}
	h := sha256.New()
	for _, name := range []string{"preview-light.svg", "preview-dark.svg", "index.html", "assets/profile.css", "assets/profile.js", "assets/newsprint-paper.svg", "assets/newsprint-folds.svg"} {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(files[name])
		h.Write([]byte{0})
	}
	wantRevision := "sha256:" + hex.EncodeToString(h.Sum(nil))
	if assetRevision != wantRevision {
		t.Fatalf("asset_revision=%q want content digest %q", assetRevision, wantRevision)
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

func TestBundleOffersSelectedWallPalettesAndNewsprintTreatment(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	files, err := Bundle(snap)
	if err != nil {
		t.Fatal(err)
	}
	html := string(files["index.html"])
	if !strings.Contains(html, `id="palettes"`) || !strings.Contains(html, `aria-label="Activity color"`) {
		t.Fatal("generated live page does not expose an activity-color switcher")
	}
	js := string(files["assets/profile.js"])
	for _, want := range []string{"Cobalt", "Magenta", "Newsprint", "ABSOLUTE_TOKEN_CAP", "wt-wall-palette"} {
		if !strings.Contains(js, want) {
			t.Fatalf("generated live script is missing %q", want)
		}
	}
	css := string(files["assets/profile.css"])
	if !strings.Contains(css, `.cell.newsprint`) {
		t.Fatal("generated live stylesheet does not provide the newsprint cell treatment")
	}
	if !strings.Contains(css, `./newsprint-paper.svg`) {
		t.Fatal("generated live stylesheet does not reference the canonical newsprint material")
	}
	if !strings.Contains(css, `./newsprint-folds.svg`) {
		t.Fatal("generated live stylesheet does not reference the canonical newsprint folds")
	}
	paper := string(files["assets/newsprint-paper.svg"])
	for _, want := range []string{"feTurbulence", "fractalNoise"} {
		if !strings.Contains(paper, want) {
			t.Fatalf("generated newsprint material is missing %q", want)
		}
	}
	folds := string(files["assets/newsprint-folds.svg"])
	for _, want := range []string{`id="vertical-fold"`, `id="diagonal-fold"`} {
		if !strings.Contains(folds, want) {
			t.Fatalf("generated newsprint folds are missing %q", want)
		}
	}
}

func TestCommittedBundlesUseEmbeddedProfileAssets(t *testing.T) {
	for _, root := range []string{
		filepath.Join("..", "..", "docs", "media", "public-profile-demo"),
		filepath.Join("..", "..", "public-profile"),
	} {
		for _, name := range []string{"index.html", "assets/profile.css", "assets/profile.js", "assets/newsprint-paper.svg", "assets/newsprint-folds.svg"} {
			got, err := os.ReadFile(filepath.Join(root, name))
			if err != nil {
				t.Fatal(err)
			}
			want, err := profilewebembed.Read(name)
			if err != nil {
				t.Fatal(err)
			}
			if !bytes.Equal(got, want) {
				t.Fatalf("%s does not match embedded source %s", root, name)
			}
		}
	}
}

func TestAssetRevisionChangesForStyleOnlyAsset(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	files, err := Bundle(snap)
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]any
	if err := json.Unmarshal(files["manifest.json"], &manifest); err != nil {
		t.Fatal(err)
	}
	snapshotID, _ := manifest["snapshot_id"].(string)
	baseRevision, _ := manifest["asset_revision"].(string)
	if snapshotID == "" || baseRevision == "" {
		t.Fatalf("manifest missing cache inputs: %#v", manifest)
	}

	styled := make(map[string][]byte, len(files))
	for name, payload := range files {
		styled[name] = append([]byte(nil), payload...)
	}
	styled["assets/profile.css"] = append(styled["assets/profile.css"], '\n')
	styleRevision := bundleAssetRevision(styled)
	if styleRevision == baseRevision {
		t.Fatal("style-only asset change did not change asset revision")
	}
	baseKey := snapshotID + "-" + strings.TrimPrefix(baseRevision, "sha256:")
	styleKey := snapshotID + "-" + strings.TrimPrefix(styleRevision, "sha256:")
	if baseKey == styleKey {
		t.Fatal("style-only asset change did not change preview cache key")
	}
}
