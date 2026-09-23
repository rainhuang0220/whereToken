package publicprofile

import (
	"strings"
	"testing"
	"time"
)

func TestShouldMaterializeReadmeCoalescesUsageAndPublishesThemeImmediately(t *testing.T) {
	now := time.Date(2026, 9, 23, 12, 0, 0, 0, time.UTC)
	if !ShouldMaterializeReadme(MaterializeInput{ThemeChange: true, Now: now, LastMaterialized: now}) {
		t.Fatal("theme change must publish immediately")
	}
	if ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: false, Now: now}) {
		t.Fatal("unchanged usage must not publish")
	}
	if !ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: true, NextTotal: 10, Now: now}) {
		t.Fatal("first usage projection may materialize")
	}
	last := now.Add(-time.Minute)
	if ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: true, PrevTotal: 0, NextTotal: 1_000_000, LastMaterialized: last, Now: now}) {
		t.Fatal("usage inside 30 minutes must wait")
	}
	last = now.Add(-ReadmeMinInterval - time.Minute)
	if !ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: true, PrevTotal: 0, NextTotal: ReadmeMinTokenDelta, LastMaterialized: last, Now: now}) {
		t.Fatal("meaningful token delta should materialize after the interval")
	}
	if ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: true, PrevTotal: 50, NextTotal: 60, LastMaterialized: last, Now: now}) {
		t.Fatal("tiny delta inside the quiet window must wait")
	}
	quiet := now.Add(-ReadmeQuietWindow - time.Minute)
	if !ShouldMaterializeReadme(MaterializeInput{SnapshotChanged: true, PrevTotal: 50, NextTotal: 60, LastMaterialized: quiet, Now: now}) {
		t.Fatal("quiet window should flush a small change")
	}
}

func TestRewriteReadmeProjectionKeepsUnrelatedContent(t *testing.T) {
	const snap = "15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62"
	const asset = "3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5"
	src := sampleReadme + "\nkeep-this-line\n<img src=\"https://github.com/rainhuang0220.png\" alt=\"portrait\">\n"
	base, err := RawPreviewBase("rainhuang0220/rainhuang0220", "main", "wheretoken")
	if err != nil {
		t.Fatal(err)
	}
	got, edits, err := RewriteReadmeProjection(src, base, "sha256:"+snap, "sha256:"+asset)
	if err != nil {
		t.Fatal(err)
	}
	if len(edits) != 3 {
		t.Fatalf("edits %d", len(edits))
	}
	key := snap + "-" + asset
	if strings.Count(got, "?v="+key) != 3 {
		t.Fatalf("cache key\n%s", got)
	}
	if strings.Contains(got, "github.io/whereToken/profile/preview") {
		t.Fatal("pages preview URL remained")
	}
	if !strings.Contains(got, "keep-this-line") || !strings.Contains(got, "rainhuang0220.png") {
		t.Fatalf("unrelated content changed\n%s", got)
	}
	if !strings.Contains(got, base+"/preview-dark.svg?v="+key) || !strings.Contains(got, base+"/preview-light.svg?v="+key) {
		t.Fatalf("preview base\n%s", got)
	}
	again, _, err := RewriteReadmeProjection(got, base, "sha256:"+snap, "sha256:"+asset)
	if err != nil {
		t.Fatal(err)
	}
	if again != got {
		t.Fatal("rewriting the same revision must be stable")
	}
}

func TestRewriteReadmeProjectionRejectsAmbiguousOrForeignURLs(t *testing.T) {
	base, err := RawPreviewBase("rainhuang0220/rainhuang0220", "main", "wheretoken")
	if err != nil {
		t.Fatal(err)
	}
	const snap = "sha256:15c96d356fc90d8073524036f29bef83ef236f76f7b02a11e862b9cbb4c52e62"
	const asset = "sha256:3e30d4babdb260f4570aa1fea89a21ff9f00376c190a0615cdf6cdbfa0d76fe5"
	if _, _, err := RewriteReadmeProjection(sampleReadme+"\n"+sampleReadme, base, snap, asset); err == nil {
		t.Fatal("ambiguous readme was accepted")
	}
	foreign := strings.ReplaceAll(sampleReadme, "https://rainhuang0220.github.io", "https://evil.example")
	if _, _, err := RewriteReadmeProjection(foreign, base, snap, asset); err == nil {
		t.Fatal("foreign preview host was accepted")
	}
	if _, err := RawPreviewBase("rainhuang0220/rainhuang0220", "main", "../wheretoken"); err == nil {
		t.Fatal("preview directory escaped")
	}
}

func TestAcceptProjectionRejectsPrivateFieldsAndKeepsIdentity(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 15, 9, 33, 58, 0, time.UTC), Loc: time.UTC, Version: "test"})
	if err != nil {
		t.Fatal(err)
	}
	raw, err := Marshal(snap)
	if err != nil {
		t.Fatal(err)
	}
	got, canonical, err := AcceptProjection(raw)
	if err != nil {
		t.Fatal(err)
	}
	if got.SnapshotID != snap.SnapshotID {
		t.Fatalf("snapshot id changed %s %s", got.SnapshotID, snap.SnapshotID)
	}
	if Sensitive(string(canonical)) {
		t.Fatal("canonical projection is sensitive")
	}
	for _, bad := range []string{
		`{"access_token":"x"}`,
		strings.Replace(string(raw), `"name": "wheretoken"`, `"name": "/Users/rainhuang"`, 1),
		strings.Replace(string(raw), `"live_sync": false`, `"live_sync": true`, 1),
	} {
		if _, _, err := AcceptProjection([]byte(bad)); err == nil {
			t.Fatalf("accepted %s", bad[:40])
		}
	}
}

func TestHostedAssetRevisionIgnoresUsageIdentity(t *testing.T) {
	snap, err := Build(Input{Now: time.Date(2026, 9, 15, 9, 33, 58, 0, time.UTC), Loc: time.UTC})
	if err != nil {
		t.Fatal(err)
	}
	id := snap.SnapshotID
	light, dark := themeBytes(t, snap, PaletteNewsprint)
	cobaltLight, cobaltDark := themeBytes(t, snap, PaletteCobalt)
	if HostedAssetRevision(light, dark, PaletteNewsprint) == HostedAssetRevision(cobaltLight, cobaltDark, PaletteCobalt) {
		t.Fatal("palette did not change the asset revision")
	}
	if snap.SnapshotID != id {
		t.Fatal("rendering changed the snapshot id")
	}
}

func themeBytes(t *testing.T, snap Snapshot, palette string) ([]byte, []byte) {
	t.Helper()
	lightTheme, darkTheme := ThemesFor(palette)
	var light, dark strings.Builder
	if err := RenderPreview(&light, snap, lightTheme); err != nil {
		t.Fatal(err)
	}
	if err := RenderPreview(&dark, snap, darkTheme); err != nil {
		t.Fatal(err)
	}
	return []byte(light.String()), []byte(dark.String())
}
