package publicprofile

import (
	"bytes"
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
	"manifest.json",
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
	man, err := json.MarshalIndent(map[string]any{
		"generated":    GeneratedFiles,
		"schema":       SchemaName,
		"snapshot_id":  snap.SnapshotID,
		"generated_at": snap.GeneratedAt,
		"as_of_date":   snap.AsOfDate,
		"provenance":   snap.Provenance.Kind,
	}, "", "  ")
	if err != nil {
		return nil, err
	}
	out["manifest.json"] = append(man, '\n')
	return out, nil
}
