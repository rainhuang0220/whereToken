package publicprofile

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"

	"github.com/rainhuang0220/whereToken/internal/profilewebembed"
)

var GeneratedFiles = []string{
	"profile.json",
	"preview-light.svg",
	"preview-dark.svg",
	"index.html",
	"assets/profile.css",
	"assets/profile.js",
	"assets/newsprint-surface.jpg",
	"presentation.json",
	"manifest.json",
}

// AssetRevisionInputs are the rendered files whose bytes invalidate GitHub's
// image cache. profile.json stays out: snapshot_id already names the data.
var AssetRevisionInputs = []string{
	"preview-light.svg",
	"preview-dark.svg",
	"index.html",
	"assets/profile.css",
	"assets/profile.js",
	"assets/newsprint-surface.jpg",
	"presentation.json",
}

// RetiredGeneratedFiles are outputs from older bundle layouts. Generators
// remove them so a rebuild cannot keep serving rejected or stale material.
var RetiredGeneratedFiles = []string{
	"assets/newsprint-paper.svg",
	"assets/newsprint-folds.svg",
	"assets/newsprint-fiber.svg",
	"assets/newsprint-fiber-b.svg",
	"assets/newsprint-surface.png",
}

func Bundle(snap Snapshot) (map[string][]byte, error) {
	return BundleWith(snap, DefaultPresentation())
}

func BundleWith(snap Snapshot, presentation Presentation) (map[string][]byte, error) {
	presentation = CoercePresentation(presentation)
	refreshSnapshotID(&snap)
	if err := Validate(snap); err != nil {
		return nil, err
	}
	out := map[string][]byte{}
	js, err := Marshal(snap)
	if err != nil {
		return nil, err
	}
	out["profile.json"] = append(js, '\n')
	lightTheme, darkTheme := ThemesFor(presentation.PublicPalette)
	var light, dark bytes.Buffer
	if err := RenderPreview(&light, snap, lightTheme); err != nil {
		return nil, err
	}
	if err := RenderPreview(&dark, snap, darkTheme); err != nil {
		return nil, err
	}
	out["preview-light.svg"] = light.Bytes()
	out["preview-dark.svg"] = dark.Bytes()
	pres, err := MarshalPresentation(presentation)
	if err != nil {
		return nil, err
	}
	out["presentation.json"] = pres
	html, err := profilewebembed.Read("index.html")
	if err != nil {
		return nil, err
	}
	css, err := profilewebembed.Read("assets/profile.css")
	if err != nil {
		return nil, err
	}
	script, err := profilewebembed.Read("assets/profile.js")
	if err != nil {
		return nil, err
	}
	out["index.html"] = html
	out["assets/profile.css"] = css
	out["assets/profile.js"] = script
	surface, err := profilewebembed.Read("assets/newsprint-surface.jpg")
	if err != nil {
		return nil, err
	}
	out["assets/newsprint-surface.jpg"] = surface
	assetRevision := bundleAssetRevision(out)
	man, err := json.MarshalIndent(map[string]any{
		"asset_revision": assetRevision,
		"generated":      GeneratedFiles,
		"schema":         SchemaName,
		"snapshot_id":    snap.SnapshotID,
		"generated_at":   snap.GeneratedAt,
		"as_of_date":     snap.AsOfDate,
		"provenance":     snap.Provenance.Kind,
		"public_palette": presentation.PublicPalette,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	out["manifest.json"] = append(man, '\n')
	return out, nil
}

func bundleAssetRevision(files map[string][]byte) string {
	h := sha256.New()
	for _, name := range AssetRevisionInputs {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(files[name])
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
