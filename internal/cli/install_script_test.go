package cli

import (
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"testing"
)

func TestInstallScriptMentionsReleaseAssets(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"wheretoken_${os}_${arch}.tar.gz",
		"github.com/rainhuang0220/whereToken",
		"go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest",
		"darwin", "linux", "amd64", "arm64",
		"checksums.txt", "sha256",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.sh missing %q", want)
		}
	}
	if strings.Contains(s, "eyJ") {
		t.Fatal("install.sh must not contain JWT material")
	}
}

func TestInstallPS1MentionsWindowsZip(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "scripts", "install.ps1"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{
		"wheretoken_windows_${goarch}.zip",
		"wheretoken.exe",
		"github.com/rainhuang0220/whereToken",
		"go install github.com/rainhuang0220/whereToken/cmd/wheretoken@latest",
		"amd64", "arm64",
		"npm install -g wheretoken",
		"checksums.txt",
		"SHA256",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("install.ps1 missing %q", want)
		}
	}
	if strings.Contains(s, "eyJ") {
		t.Fatal("install.ps1 must not contain JWT material")
	}
}

func TestGoreleaserShipsManCompletionsAndLicense(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"LICENSE", "README.md", "CHANGELOG.md", "docs/wheretoken.1", "docs/cli-json.schema.json", "completions/*", "nfpms:", "deb", "rpm"} {
		if !strings.Contains(s, want) {
			t.Errorf("goreleaser missing %q", want)
		}
	}
}

func TestGitHubWorkflowsPinActionSHAs(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows")
	sha := regexp.MustCompile(`^[0-9a-f]{40}$`)
	uses := regexp.MustCompile(`uses:\s+(\S+)`)
	for _, name := range []string{"ci.yml", "release.yml"} {
		body, err := os.ReadFile(filepath.Join(root, name))
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range uses.FindAllStringSubmatch(string(body), -1) {
			ref := m[1]
			_, at, ok := strings.Cut(ref, "@")
			if !ok {
				t.Errorf("%s: uses %s has no ref", name, ref)
				continue
			}
			if !sha.MatchString(at) {
				t.Errorf("%s: unpinned action %s", name, ref)
			}
		}
	}
}

func TestCIRunsGofmt(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "ci", "github-workflows", "ci.yml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(body), "gofmt -l") {
		t.Fatal("ci.yml should fail the build when gofmt -l is non-empty")
	}
}

func TestHomebrewFormulaIsHeadBuild(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "Formula", "wheretoken.rb"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{`class Wheretoken`, `head "https://github.com/rainhuang0220/whereToken.git"`, "./cmd/wheretoken", "docs/wheretoken.1", "bash_completion", "zsh_completion", "fish_completion"} {
		if !strings.Contains(s, want) {
			t.Errorf("formula missing %q", want)
		}
	}
}

func TestManPageMentionsJSONSchema(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	body, err := os.ReadFile(filepath.Join(filepath.Dir(file), "..", "..", "docs", "wheretoken.1"))
	if err != nil {
		t.Fatal(err)
	}
	s := string(body)
	for _, want := range []string{"cli-json.schema.json", "JWT", "127.0.0.1", "--offline", "--width"} {
		if !strings.Contains(s, want) {
			t.Errorf("man page missing %q", want)
		}
	}
}
