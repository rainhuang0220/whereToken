package credstore

import (
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestKeychainRoundTrip(t *testing.T) {
	mem := map[string]string{}
	k := &cmdStore{
		service: "whereToken-test",
		run: func(args []string) (string, error) {
			switch args[0] {
			case "add-generic-password":
				mem[flagVal(args, "-a")] = flagVal(args, "-w")
				return "", nil
			case "find-generic-password":
				acc := flagVal(args, "-a")
				if v, ok := mem[acc]; ok {
					return v + "\n", nil
				}
				return "", errors.New("not found")
			case "delete-generic-password":
				delete(mem, flagVal(args, "-a"))
				return "", nil
			default:
				return "", errors.New("unknown")
			}
		},
	}
	if err := k.Set(KeyDeviceToken, "wtd_1.secret"); err != nil {
		t.Fatal(err)
	}
	got, err := k.Get(KeyDeviceToken)
	if err != nil || got != "wtd_1.secret" {
		t.Fatalf("got %q %v", got, err)
	}
	if err := k.Delete(KeyDeviceToken); err != nil {
		t.Fatal(err)
	}
	if _, err := k.Get(KeyDeviceToken); err != ErrNotFound {
		t.Fatal("expected missing")
	}
}

func TestChainPrefersOSStoreAndFallsBackToFile(t *testing.T) {
	dir := t.TempDir()
	file := DirStore(dir)
	osb := DirStore(t.TempDir())
	s := &chain{os: osb, file: file}

	if err := file.Set(KeyDeviceToken, "from-file"); err != nil {
		t.Fatal(err)
	}
	got, err := s.Get(KeyDeviceToken)
	if err != nil || got != "from-file" {
		t.Fatalf("migration get %q %v", got, err)
	}
	if err := s.Set(KeyDeviceToken, "from-os"); err != nil {
		t.Fatal(err)
	}
	if v, _ := osb.Get(KeyDeviceToken); v != "from-os" {
		t.Fatalf("os store %q", v)
	}
	if _, err := file.Get(KeyDeviceToken); err != ErrNotFound {
		t.Fatal("file copy should be removed after keychain write")
	}
}

func TestChainSetFallsBackWhenOSStoreFails(t *testing.T) {
	dir := t.TempDir()
	file := DirStore(dir)
	s := &chain{os: failStore{}, file: file}
	if err := s.Set(KeyHMAC, "hmac-value"); err != nil {
		t.Fatal(err)
	}
	st, err := os.Stat(filepath.Join(dir, KeyHMAC))
	if err != nil {
		t.Fatal(err)
	}
	if IsUnix() && st.Mode().Perm() != 0o600 {
		t.Fatalf("mode %o", st.Mode().Perm())
	}
}

func flagVal(args []string, name string) string {
	for i := 0; i < len(args)-1; i++ {
		if args[i] == name {
			return args[i+1]
		}
	}
	return ""
}

type failStore struct{}

func (failStore) Get(string) (string, error) { return "", errors.New("no os store") }
func (failStore) Set(string, string) error   { return errors.New("no os store") }
func (failStore) Delete(string) error        { return errors.New("no os store") }

var _ Store = (*cmdStore)(nil)
var _ Store = (*chain)(nil)
