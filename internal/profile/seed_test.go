package profile

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
	"github.com/rainhuang0220/whereToken/internal/community"
)

func TestIdentityCreatesStableInstallID(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	dir := t.TempDir()

	id1 := Identity(dir)
	if id1 == "" || id1 == FallbackSeed {
		t.Fatalf("identity must be a real id, got %q", id1)
	}
	if !installIDRe.MatchString(id1) {
		t.Fatalf("install id %q is not a UUIDv4-shaped string", id1)
	}
	// The config dir follows community.ConfigPath (XDG on unix, AppData on
	// Windows) — derive it instead of hardcoding a unix layout.
	path := filepath.Join(filepath.Dir(community.ConfigPath(testhome.New(dir))), "install-id")
	st, err := os.Stat(path)
	if err != nil {
		t.Fatalf("install-id not written: %v", err)
	}
	if perm := st.Mode().Perm(); runtime.GOOS != "windows" && perm != 0o600 {
		// Windows maps write modes to 0666/0444; the 0600 intent is posix-only.
		t.Fatalf("install-id mode=%o, want 0600", perm)
	}
	if id2 := Identity(dir); id2 != id1 {
		t.Fatalf("identity must be stable: %q then %q", id1, id2)
	}
	// No PII: the id must not embed the home path or any part of it.
	if strings.Contains(id1, dir) || strings.Contains(id1, filepath.Base(dir)) {
		t.Fatalf("identity leaks path: %q", id1)
	}
}

func TestIdentityPrefersCommunityParticipantID(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	dir := t.TempDir()

	pid, err := community.NewParticipantID()
	if err != nil {
		t.Fatal(err)
	}
	f := &community.File{ParticipantID: pid, Enabled: true, JoinedAt: "2026-09-03T00:00:00Z"}
	if err := f.Save(community.ConfigPath(testhome.New(dir))); err != nil {
		t.Fatal(err)
	}
	if got := Identity(dir); got != pid {
		t.Fatalf("Identity=%q, want participant_id %q", got, pid)
	}
	// The participant id already identifies the install; no install-id file.
	if _, err := os.Stat(filepath.Join(dir, ".config", "wheretoken", "install-id")); !os.IsNotExist(err) {
		t.Fatalf("install-id should not be created when participant_id exists: %v", err)
	}
}

func TestIdentityRegeneratesCorruptInstallID(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	dir := t.TempDir()

	cfgDir := filepath.Join(dir, ".config", "wheretoken")
	if err := os.MkdirAll(cfgDir, 0o700); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(cfgDir, "install-id")
	if err := os.WriteFile(path, []byte("garbage\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	id := Identity(dir)
	if id == "garbage" || !installIDRe.MatchString(id) {
		t.Fatalf("corrupt install-id must be replaced, got %q", id)
	}
	if id2 := Identity(dir); id2 != id {
		t.Fatalf("regenerated id must persist: %q then %q", id, id2)
	}
}

func TestIdentityNeverReturnsEmpty(t *testing.T) {
	t.Setenv("WHERETOKEN_COMMUNITY_FILE", "")
	if got := Identity(""); got == "" {
		t.Fatal("Identity(\"\") returned empty")
	}
	if got := IdentityFor(nil); got == "" {
		t.Fatal("IdentityFor(nil) returned empty")
	}
	// The no-home fallback must not touch the filesystem: it is a constant.
	if Identity("") != FallbackSeed || IdentityFor(nil) != FallbackSeed {
		t.Fatal("no-home identity should be the constant fallback seed")
	}
}
