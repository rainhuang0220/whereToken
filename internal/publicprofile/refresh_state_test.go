package publicprofile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestRefreshStateDropsUnknownFields(t *testing.T) {
	home := testhome.New(t.TempDir())
	path := RefreshStatePath(home)
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		t.Fatal(err)
	}
	const raw = `{
  "schema_version": 1,
  "phase": "published",
  "code": "PUBLISHED",
  "checked_at": "2026-09-25T08:00:00Z",
  "total": 10,
  "snapshot_id": "sha256:nope",
  "path": "/Users/hidden"
}
`
	if err := os.WriteFile(path, []byte(raw), 0o600); err != nil {
		t.Fatal(err)
	}
	st, err := LoadRefreshState(path)
	if err != nil || st.Code != CodePublished || st.Phase != PhasePublished {
		t.Fatalf("%+v %v", st, err)
	}
	if err := SaveRefreshState(path, st); err != nil {
		t.Fatal(err)
	}
	saved, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	text := string(saved)
	for _, bad := range []string{"snapshot_id", "/Users", "total", "sha256"} {
		if strings.Contains(text, bad) {
			t.Fatalf("saved %q in %s", bad, text)
		}
	}
	if strings.Contains(text, "00:00:00") {
		t.Fatalf("checked_at drifted to midnight: %s", text)
	}
	if !strings.Contains(text, "2026-09-25T08:00:00Z") {
		t.Fatalf("checked_at %s", text)
	}
	if runtime.GOOS != "windows" {
		info, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		if info.Mode().Perm() != 0o600 {
			t.Fatalf("mode %o", info.Mode().Perm())
		}
	}
	if err := SaveRefreshState(path, RefreshState{Phase: PhasePublished, Code: "0.00 M", CheckedAt: time.Date(2026, 9, 25, 8, 0, 0, 0, time.UTC)}); err != nil {
		t.Fatal(err)
	}
	st, err = LoadRefreshState(path)
	if err != nil || st.Code != "" {
		t.Fatalf("disallowed code persisted: %+v %v", st, err)
	}
}
