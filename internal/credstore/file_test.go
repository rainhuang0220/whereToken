package credstore

import (
	"os"
	"path/filepath"
	"testing"
)

func TestFileStoreRoundTripAndMode(t *testing.T) {
	dir := t.TempDir()
	s := DirStore(dir)
	if _, err := s.Get(KeyDeviceToken); err != ErrNotFound {
		t.Fatalf("empty get: %v", err)
	}
	if err := s.Set(KeyDeviceToken, "wtd_1.secret"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(KeyDeviceToken)
	if err != nil || got != "wtd_1.secret" {
		t.Fatalf("got %q %v", got, err)
	}
	if IsUnix() {
		st, err := os.Stat(filepath.Join(dir, KeyDeviceToken))
		if err != nil {
			t.Fatal(err)
		}
		if st.Mode().Perm() != 0o600 {
			t.Fatalf("mode %o", st.Mode().Perm())
		}
	}
	if err := s.Delete(KeyDeviceToken); err != nil {
		t.Fatal(err)
	}
	if _, err := s.Get(KeyDeviceToken); err != ErrNotFound {
		t.Fatal("deleted token still present")
	}
}
