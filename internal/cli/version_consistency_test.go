package cli

import (
	"encoding/json"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

// The git tag is the single source of truth for a release; CHANGELOG.md's top
// heading mirrors it. These tests pin the hand-edited copies (Homebrew formula,
// npm wrapper, public site) to that version so they cannot drift again.
// docs/releasing.md is the checklist these tests back up.

func versionConsistencyRoot(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(file), "..", "..")
}

func latestChangelogVersion(t *testing.T, root string) string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+) —`).FindStringSubmatch(string(body))
	if m == nil {
		t.Fatal("CHANGELOG.md has no `## X.Y.Z —` version heading")
	}
	return m[1]
}

func TestHomebrewFormulaTracksLatestRelease(t *testing.T) {
	root := versionConsistencyRoot(t)
	latest := latestChangelogVersion(t, root)
	body, err := os.ReadFile(filepath.Join(root, "Formula", "wheretoken.rb"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`url "([^"]+)"`).FindStringSubmatch(string(body))
	if m == nil {
		t.Fatal("Formula/wheretoken.rb has no url")
	}
	if !strings.Contains(m[1], "v"+latest) {
		t.Errorf("Formula/wheretoken.rb url %q is stale: CHANGELOG.md top release is %s; bump url + sha256 per docs/releasing.md", m[1], latest)
	}
}

func TestNpmPackageVersionTracksLatestRelease(t *testing.T) {
	root := versionConsistencyRoot(t)
	latest := latestChangelogVersion(t, root)
	body, err := os.ReadFile(filepath.Join(root, "npm", "package.json"))
	if err != nil {
		t.Fatal(err)
	}
	var pkg struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal(body, &pkg); err != nil {
		t.Fatal(err)
	}
	if pkg.Version != latest {
		t.Errorf("npm/package.json version %q != CHANGELOG.md top release %s: install.js fetches the GitHub release for v<pkg.version>, so it must track the release", pkg.Version, latest)
	}
}

func TestSiteDownloadButtonIsVersionFree(t *testing.T) {
	root := versionConsistencyRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "site", "index.html"))
	if err != nil {
		t.Fatal(err)
	}
	if m := regexp.MustCompile(`Download v\d+\.\d+\.\d+`).FindString(string(body)); m != "" {
		t.Errorf("site/index.html hardcodes %q: the href already points at releases/latest, keep the label version-free", m)
	}
}
