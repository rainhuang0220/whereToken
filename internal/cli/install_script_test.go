package cli

import (
	"os"
	"path/filepath"
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
	for _, want := range []string{"LICENSE", "README.md", "CHANGELOG.md", "docs/wheretoken.1", "completions/*"} {
		if !strings.Contains(s, want) {
			t.Errorf("goreleaser missing %q", want)
		}
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
	for _, want := range []string{`class Wheretoken`, `head "https://github.com/rainhuang0220/whereToken.git"`, "./cmd/wheretoken", "docs/wheretoken.1", "bash_completion", "zsh_completion"} {
		if !strings.Contains(s, want) {
			t.Errorf("formula missing %q", want)
		}
	}
}
