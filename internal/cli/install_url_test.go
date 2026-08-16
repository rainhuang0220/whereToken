package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestReleaseAssetNamesMatchGoreleaserAndNpm(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..")
	goreleaser, err := os.ReadFile(filepath.Join(root, ".goreleaser.yaml"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(goreleaser), `name_template: "{{ .ProjectName }}_{{ .Os }}_{{ .Arch }}"`) {
		t.Fatal("goreleaser archive name_template drifted from install.sh/npm")
	}
	npm, err := os.ReadFile(filepath.Join(root, "npm", "lib", "platform.js"))
	if err != nil {
		t.Fatal(err)
	}
	js := string(npm)
	if !strings.Contains(js, "wheretoken_${os}_${goarch}.${ext}") {
		t.Fatal("npm githubAsset name drifted from goreleaser")
	}
	if !strings.Contains(js, "windows") || !strings.Contains(js, "zip") {
		t.Fatal("npm must still emit windows zip names")
	}
	sh, err := os.ReadFile(filepath.Join(root, "scripts", "install.sh"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(sh), `asset="wheretoken_${os}_${arch}.tar.gz"`) {
		t.Fatal("install.sh asset name drifted from goreleaser")
	}
}
