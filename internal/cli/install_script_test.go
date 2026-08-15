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
