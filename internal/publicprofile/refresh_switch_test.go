package publicprofile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestRefreshSwitchIgnoresUnknownFields(t *testing.T) {
	home := testhome.New(t.TempDir())
	path := RefreshConfigPath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const raw = `{
  "schema_version": 1,
  "enabled": true,
  "snapshot_id": "sha256:nope",
  "token": "secret",
  "path": "/Users/hidden"
}
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	on, err := LoadRefreshSwitch(path)
	if err != nil || !on {
		t.Fatalf("load %v %v", on, err)
	}
	if err := SaveRefreshSwitch(path, true); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(saved)
	for _, bad := range []string{"snapshot_id", "secret", "/Users", "token"} {
		if strings.Contains(text, bad) {
			t.Fatalf("saved %q in %s", bad, text)
		}
	}
	if runtime.GOOS != "windows" {
		st, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %o", st.Mode().Perm())
		}
	}
}
