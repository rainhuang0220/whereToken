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

func changelogVersions(t *testing.T, root string) []string {
	t.Helper()
	body, err := os.ReadFile(filepath.Join(root, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	ms := regexp.MustCompile(`(?m)^## (\d+\.\d+\.\d+) —`).FindAllStringSubmatch(string(body), -1)
	if len(ms) == 0 {
		t.Fatal("CHANGELOG.md has no `## X.Y.Z —` version heading")
	}
	out := make([]string, 0, len(ms))
	for _, m := range ms {
		out = append(out, m[1])
	}
	return out
}

func latestChangelogVersion(t *testing.T, root string) string {
	t.Helper()
	return changelogVersions(t, root)[0]
}

func TestHomebrewFormulaTracksLatestRelease(t *testing.T) {
	root := versionConsistencyRoot(t)
	body, err := os.ReadFile(filepath.Join(root, "Formula", "wheretoken.rb"))
	if err != nil {
		t.Fatal(err)
	}
	m := regexp.MustCompile(`url "([^"]+)"`).FindStringSubmatch(string(body))
	if m == nil {
		t.Fatal("Formula/wheretoken.rb has no url")
	}
	// The formula builds from the tag tarball, whose sha256 exists only after
	// the tag — so docs/releasing.md bumps it right after release. It may lag
	// the CHANGELOG top by exactly one release, never more.
	allowed := changelogVersions(t, root)
	if len(allowed) > 2 {
		allowed = allowed[:2]
	}
	for _, v := range allowed {
		if strings.Contains(m[1], "v"+v) {
			return
		}
	}
	t.Errorf("Formula/wheretoken.rb url %q is stale: CHANGELOG.md recent releases are %v; bump url + sha256 per docs/releasing.md", m[1], allowed)
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
