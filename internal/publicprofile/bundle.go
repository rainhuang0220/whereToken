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
	"assets/newsprint-surface.png",
	"assets/newsprint-fiber.svg",
	"assets/newsprint-fiber-b.svg",
	"manifest.json",
}

// RetiredGeneratedFiles are outputs from older bundle layouts. Generators
// remove them so a rebuild cannot keep serving rejected or stale material.
var RetiredGeneratedFiles = []string{
	"assets/newsprint-paper.svg",
	"assets/newsprint-folds.svg",
}

func Bundle(snap Snapshot) (map[string][]byte, error) {
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
	var light, dark bytes.Buffer
	if err := RenderPreview(&light, snap, ThemeLight); err != nil {
		return nil, err
	}
	if err := RenderPreview(&dark, snap, ThemeDark); err != nil {
		return nil, err
	}
	out["preview-light.svg"] = light.Bytes()
	out["preview-dark.svg"] = dark.Bytes()
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
	surface, err := profilewebembed.Read("assets/newsprint-surface.png")
	if err != nil {
		return nil, err
	}
	fiber, err := profilewebembed.Read("assets/newsprint-fiber.svg")
	if err != nil {
		return nil, err
	}
	fiberB, err := profilewebembed.Read("assets/newsprint-fiber-b.svg")
	if err != nil {
		return nil, err
	}
	out["assets/newsprint-surface.png"] = surface
	out["assets/newsprint-fiber.svg"] = fiber
	out["assets/newsprint-fiber-b.svg"] = fiberB
	assetRevision := bundleAssetRevision(out)
	man, err := json.MarshalIndent(map[string]any{
		"asset_revision": assetRevision,
		"generated":      GeneratedFiles,
		"schema":         SchemaName,
		"snapshot_id":    snap.SnapshotID,
		"generated_at":   snap.GeneratedAt,
		"as_of_date":     snap.AsOfDate,
		"provenance":     snap.Provenance.Kind,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	out["manifest.json"] = append(man, '\n')
	return out, nil
}

func bundleAssetRevision(files map[string][]byte) string {
	h := sha256.New()
	for _, name := range []string{"preview-light.svg", "preview-dark.svg", "index.html", "assets/profile.css", "assets/profile.js", "assets/newsprint-surface.png", "assets/newsprint-fiber.svg", "assets/newsprint-fiber-b.svg"} {
		h.Write([]byte(name))
		h.Write([]byte{0})
		h.Write(files[name])
		h.Write([]byte{0})
	}
	return "sha256:" + hex.EncodeToString(h.Sum(nil))
}
