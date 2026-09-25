package proclock

import (
	"os"
	"path/filepath"
	"testing"
)

func TestTryLockIsExclusiveAndEmpty(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cfg", "scan.lock")
	release, ok, err := TryLock(path)
	if err != nil || !ok {
		t.Fatalf("first ok=%v err=%v", ok, err)
	}
	_, ok, err = TryLock(path)
	if err != nil || ok {
		release()
		t.Fatalf("second ok=%v err=%v", ok, err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		release()
		t.Fatal(err)
	}
	if len(raw) != 0 {
		release()
		t.Fatalf("lock file stored %q", raw)
	}
	release()
	release, ok, err = TryLock(path)
	if err != nil || !ok {
		t.Fatalf("after unlock ok=%v err=%v", ok, err)
	}
	release()
}
