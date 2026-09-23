package publicprofile

import (
	"encoding/json"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestWriteBundleKeepsSnapshotIdentity(t *testing.T) {
	dir := t.TempDir()
	snap, err := Build(Input{Now: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	if err := WriteBundle(dir, snap, DefaultPresentation()); err != nil {
		t.Fatal(err)
	}
	before, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	next := Presentation{SchemaVersion: 1, PublicPalette: PaletteMagenta, Revision: "2", UpdatedAt: "2026-09-23T00:00:00Z"}
	if err := WriteBundle(dir, before, next); err != nil {
		t.Fatal(err)
	}
	after, err := LoadFile(dir)
	if err != nil {
		t.Fatal(err)
	}
	if before.SnapshotID == "" || before.SnapshotID != after.SnapshotID {
		t.Fatalf("snapshot id changed %s -> %s", before.SnapshotID, after.SnapshotID)
	}
	got, err := ReadBundlePresentation(dir)
	if err != nil {
		t.Fatal(err)
	}
	if got.PublicPalette != PaletteMagenta || got.Revision != "2" {
		t.Fatalf("%+v", got)
	}
	raw, err := os.ReadFile(filepath.Join(dir, "presentation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "bundle_dir") {
		t.Fatal("bundle presentation contains a local path field")
	}
}

func TestOwnerConfigPathFollowsPlatform(t *testing.T) {
	t.Setenv("WHERETOKEN_PUBLIC_PROFILE_FILE", "")
	home := testhome.New(t.TempDir())
	unix := filepath.Join(home.XDGConfig("wheretoken"), "public-profile.json")
	win := filepath.Join(home.AppData("whereToken"), "public-profile.json")
	if unix == win {
		t.Fatal("testhome XDGConfig and AppData public-profile paths must differ")
	}
	want := unix
	if runtime.GOOS == "windows" {
		want = win
	}
	if got := ConfigPath(home); got != want {
		t.Fatalf("ConfigPath=%q want %q (unix=%q windows=%q)", got, want, unix, win)
	}
	if got := ConfigPathIn(home); got != want {
		t.Fatalf("ConfigPathIn=%q want %q", got, want)
	}
	custom := filepath.Join(t.TempDir(), "override.json")
	t.Setenv("WHERETOKEN_PUBLIC_PROFILE_FILE", custom)
	if got := ConfigPath(home); got != custom {
		t.Fatalf("ConfigPath=%q want override %q", got, custom)
	}
	if got := ConfigPathIn(home); got != want {
		t.Fatalf("ConfigPathIn must ignore the file override, got %q", got)
	}
}

func TestPaletteRejectsUnknownAndCoercesSafely(t *testing.T) {
	if err := ValidatePalette("newsprint"); err != nil {
		t.Fatal(err)
	}
	if err := ValidatePalette("kiln"); err == nil {
		t.Fatal("dashboard glaze must not be a public palette")
	}
	got := CoercePresentation(Presentation{SchemaVersion: 9, PublicPalette: "nope", Revision: "4"})
	if got.PublicPalette != DefaultPalette || got.SchemaVersion != PresentationSchema {
		t.Fatalf("coerce %+v", got)
	}
	kept := CoercePresentation(Presentation{SchemaVersion: 1, PublicPalette: "cobalt", Revision: "3", UpdatedAt: "2026-09-23T00:00:00Z"})
	if kept.PublicPalette != "cobalt" || kept.Revision != "3" || kept.UpdatedAt == "" {
		t.Fatalf("kept %+v", kept)
	}
}

func TestOwnerPaletteRoundTripDoesNotLeakBundleDirIntoPublicFile(t *testing.T) {
	home := testhome.New(t.TempDir())
	path := ConfigPath(home)
	owner := DefaultOwner()
	if err := owner.ApplyPalette("magenta", "2026-09-23T01:02:03Z"); err != nil {
		t.Fatal(err)
	}
	owner.BundleDir = filepath.Join(t.TempDir(), "secret-bundle")
	if err := SaveOwner(path, owner); err != nil {
		t.Fatal(err)
	}
	loaded, err := LoadOwner(path)
	if err != nil {
		t.Fatal(err)
	}
	if loaded.PublicPalette != "magenta" || loaded.Revision != "1" || loaded.BundleDir != owner.BundleDir {
		t.Fatalf("%+v", loaded)
	}
	raw, err := MarshalPresentation(loaded.Presentation())
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(raw), "secret-bundle") || strings.Contains(string(raw), "bundle_dir") {
		t.Fatalf("public presentation leaked local path: %s", raw)
	}
	if _, err := LoadOwner(filepath.Join(t.TempDir(), "missing.json")); !os.IsNotExist(err) {
		t.Fatal(err)
	}
	fb := FallbackOwner(filepath.Join(t.TempDir(), "missing.json"))
	if fb.PublicPalette != DefaultPalette || OwnerSaved(fb) {
		t.Fatalf("fallback %+v", fb)
	}
}

func TestPublicationStatusSeparatesSaveFromBundle(t *testing.T) {
	owner := DefaultOwner()
	got := DescribePublication(owner, "", Presentation{}, false, "")
	if got.Status != StatusUnconfigured || got.PublicPalette != "newsprint" {
		t.Fatalf("%+v", got)
	}
	if err := owner.ApplyPalette("cobalt", "2026-09-23T01:02:03Z"); err != nil {
		t.Fatal(err)
	}
	saved := DescribePublication(owner, "", Presentation{}, false, "")
	if saved.Status != StatusSavedLocally {
		t.Fatalf("saved %+v", saved)
	}
	pending := DescribePublication(owner, "/tmp/profile", DefaultPresentation(), true, "")
	if pending.Status != StatusPending || pending.PreviewPath == "" {
		t.Fatalf("pending %+v", pending)
	}
	ready := DescribePublication(owner, "/tmp/profile", owner.Presentation(), true, "")
	if ready.Status != StatusReadyToPublish {
		t.Fatalf("ready %+v", ready)
	}
	if strings.Contains(ready.Status, "published") && ready.Status != StatusReadyToPublish {
		t.Fatal("status must not claim remote publication")
	}
	failed := DescribePublication(owner, "/tmp/profile", owner.Presentation(), true, "could not write the local public bundle")
	if failed.Status != StatusFailed || failed.Error == "" {
		t.Fatalf("failed %+v", failed)
	}
}

func TestBundlePaletteChangesPreviewsWithoutChangingSnapshot(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 13, 12, 0, 0, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	ser := allTokenSeries(&snap)
	if ser == nil || len(ser.Values) < 2 || len(ser.States) < 2 {
		t.Fatal("missing activity series")
	}
	ser.States[0], ser.Values[0] = CellActive, 200_000_000
	ser.States[1], ser.Values[1] = CellActive, 800_000_000
	base, err := Bundle(snap)
	if err != nil {
		t.Fatal(err)
	}
	cobalt, err := BundleWith(snap, Presentation{SchemaVersion: 1, PublicPalette: "cobalt", Revision: "4", UpdatedAt: "2026-09-23T01:02:03Z"})
	if err != nil {
		t.Fatal(err)
	}
	var baseProfile, cobaltProfile map[string]any
	if err := json.Unmarshal(base["profile.json"], &baseProfile); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cobalt["profile.json"], &cobaltProfile); err != nil {
		t.Fatal(err)
	}
	if baseProfile["snapshot_id"] == "" || baseProfile["snapshot_id"] != cobaltProfile["snapshot_id"] {
		t.Fatalf("palette change moved snapshot id %v -> %v", baseProfile["snapshot_id"], cobaltProfile["snapshot_id"])
	}
	if string(base["preview-light.svg"]) == string(cobalt["preview-light.svg"]) {
		t.Fatal("light preview ignored the public palette")
	}
	if string(base["preview-dark.svg"]) == string(cobalt["preview-dark.svg"]) {
		t.Fatal("dark preview ignored the public palette")
	}
	if strings.Contains(string(cobalt["preview-light.svg"]), "newsprint-ink-") {
		t.Fatal("cobalt light preview used the newsprint ink pattern")
	}
	if !strings.Contains(string(base["preview-light.svg"]), "newsprint-ink-") {
		t.Fatal("newsprint light preview lost ink variation")
	}
	if !strings.Contains(string(base["preview-dark.svg"]), "newsprint-ink-") {
		t.Fatal("newsprint dark preview lost its ink treatment")
	}
	var pres map[string]any
	if err := json.Unmarshal(cobalt["presentation.json"], &pres); err != nil {
		t.Fatal(err)
	}
	if pres["public_palette"] != "cobalt" || pres["revision"] != "4" {
		t.Fatalf("presentation %+v", pres)
	}
	var baseMan, cobaltMan map[string]any
	if err := json.Unmarshal(base["manifest.json"], &baseMan); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(cobalt["manifest.json"], &cobaltMan); err != nil {
		t.Fatal(err)
	}
	if baseMan["asset_revision"] == cobaltMan["asset_revision"] {
		t.Fatal("palette change did not move asset revision")
	}
	if baseMan["snapshot_id"] != cobaltMan["snapshot_id"] {
		t.Fatal("manifest snapshot id changed with palette")
	}
	bad, err := BundleWith(snap, Presentation{SchemaVersion: 4, PublicPalette: "kiln"})
	if err != nil {
		t.Fatal(err)
	}
	var badPres map[string]any
	if err := json.Unmarshal(bad["presentation.json"], &badPres); err != nil {
		t.Fatal(err)
	}
	if badPres["public_palette"] != DefaultPalette {
		t.Fatalf("invalid palette published as %v", badPres["public_palette"])
	}
}
