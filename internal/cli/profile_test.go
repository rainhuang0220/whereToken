package cli

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/publicprofile"
)

func TestParseProfileBuildAndValidate(t *testing.T) {
	f, err := Parse([]string{"profile", "build", "out"})
	if err != nil {
		t.Fatal(err)
	}
	if f.Command != CommandProfile || f.ProfileAction != "build" || f.ProfilePath != "out" {
		t.Fatalf("%+v", f)
	}
	f, err = Parse([]string{"--offline", "--quiet", "profile", "validate", "out/profile.json"})
	if err != nil || !f.Offline || !f.Quiet || f.ProfileAction != "validate" {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = Parse([]string{"profile", "build", "out", "--include-models", "--include-cost"})
	if err != nil || !f.IncludeModels || !f.IncludeCost {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = Parse([]string{"profile", "validate", "out", "--production"})
	if err != nil || !f.Production || f.AllowPartial {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = Parse([]string{"profile", "validate", "out", "--production", "--allow-partial"})
	if err != nil || !f.Production || !f.AllowPartial {
		t.Fatalf("%+v %v", f, err)
	}
	if _, err := Parse([]string{"profile", "build", "out", "--production"}); err == nil || !IsUsage(err) {
		t.Fatalf("build production: %v", err)
	}
	if _, err := Parse([]string{"profile", "validate", "out", "--allow-partial"}); err == nil || !IsUsage(err) {
		t.Fatalf("allow-partial alone: %v", err)
	}
	if _, err := Parse([]string{"profile", "build", "out", "--today"}); err == nil || !IsUsage(err) {
		t.Fatalf("today: %v", err)
	}
	if _, err := Parse([]string{"profile"}); err == nil || !IsUsage(err) {
		t.Fatalf("bare: %v", err)
	}
}

func TestRunProfileBuildPreservesUserFiles(t *testing.T) {
	dir := t.TempDir()
	notes := filepath.Join(dir, "user-notes.txt")
	if err := os.WriteFile(notes, []byte("keep-me"), 0o644); err != nil {
		t.Fatal(err)
	}
	app, stdout, stderr := testApp([]string{"profile", "build", dir, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d stderr=%s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "wrote ") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	got, err := os.ReadFile(notes)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != "keep-me" {
		t.Fatalf("user file mutated: %q", got)
	}
	if _, err := os.Stat(filepath.Join(dir, "profile.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "preview-light.svg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "preview-dark.svg")); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(dir, "index.html")); err != nil {
		t.Fatal(err)
	}
	js, err := os.ReadFile(filepath.Join(dir, "assets/profile.js"))
	if err != nil {
		t.Fatal(err)
	}
	for _, bad := range []string{"/api/summary", "/api/v1/", "wheretoken login", "wheretoken sync"} {
		if strings.Contains(string(js), bad) {
			t.Fatalf("live page contains %q", bad)
		}
	}
	if err := publicprofile.ValidateFile(dir); err != nil {
		t.Fatal(err)
	}
}

func TestProfilePaletteFeedsTheNextBuild(t *testing.T) {
	home := t.TempDir()
	app, stdout, stderr := testApp([]string{"--home", home, "profile", "palette", "cobalt"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("save %d %s", code, stderr.String())
	}
	if !strings.Contains(stdout.String(), "saved public_palette=cobalt") {
		t.Fatalf("stdout=%q", stdout.String())
	}
	dir := t.TempDir()
	app, _, stderr = testApp([]string{"--home", home, "--quiet", "profile", "build", dir})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("build %d %s", code, stderr.String())
	}
	raw, err := os.ReadFile(filepath.Join(dir, "presentation.json"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(raw), `"public_palette": "cobalt"`) {
		t.Fatalf("presentation=%s", raw)
	}
	light, err := os.ReadFile(filepath.Join(dir, "preview-light.svg"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(light), "newsprint-ink-") {
		t.Fatal("cobalt build still rendered newsprint ink")
	}
	if _, err := Parse([]string{"profile", "palette", "kiln"}); err == nil || !IsUsage(err) {
		t.Fatalf("kiln palette: %v", err)
	}
	if _, err := Parse([]string{"profile", "validate", "out", "--public-palette", "cobalt"}); err == nil || !IsUsage(err) {
		t.Fatalf("validate palette flag: %v", err)
	}
}

func TestRunProfileValidate(t *testing.T) {
	dir := t.TempDir()
	app, _, stderr := testApp([]string{"profile", "build", dir, "--quiet"})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("build %d %s", code, stderr.String())
	}
	app, stdout, stderr := testApp([]string{"profile", "validate", dir})
	if code := app.Run(); code != ExitOK {
		t.Fatalf("validate %d %s", code, stderr.String())
	}
	if strings.TrimSpace(stdout.String()) != "ok" {
		t.Fatalf("stdout=%q", stdout.String())
	}
}
