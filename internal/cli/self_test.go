package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestParseUpdateAndUninstall(t *testing.T) {
	f, err := Parse([]string{"update"})
	if err != nil || f.Command != CommandUpdate {
		t.Fatalf("%+v %v", f, err)
	}
	f, err = Parse([]string{"upgrade"})
	if err != nil || f.Command != CommandUpdate {
		t.Fatalf("upgrade alias %+v %v", f, err)
	}
	f, err = Parse([]string{"uninstall"})
	if err != nil || f.Command != CommandUninstall {
		t.Fatalf("%+v %v", f, err)
	}
}

func TestRunUninstallRemovesBinary(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "wheretoken")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, _, errb := testApp([]string{"uninstall"})
	app.Executable = func() (string, error) { return dest, nil }
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if _, err := os.Stat(dest); !os.IsNotExist(err) {
		t.Fatalf("still there: %v", err)
	}
	if !strings.Contains(errb.String(), "removed") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunUpdateReplacesBinary(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "wheretoken")
	if err := os.WriteFile(dest, []byte("old-bin"), 0o755); err != nil {
		t.Fatal(err)
	}
	payload := []byte("new-bin-contents")
	archive := tarGzNamed(t, "wheretoken", payload)
	sum := sha256.Sum256(archive)
	sums := hex.EncodeToString(sum[:]) + "  wheretoken_darwin_arm64.tar.gz\n"
	app, _, errb := testApp([]string{"update"})
	app.GOOS = "darwin"
	app.GOARCH = "arm64"
	app.Executable = func() (string, error) { return dest, nil }
	app.HTTPGet = func(url string) ([]byte, error) {
		switch {
		case strings.HasSuffix(url, "checksums.txt"):
			return []byte(sums), nil
		case strings.HasSuffix(url, "wheretoken_darwin_arm64.tar.gz"):
			return archive, nil
		default:
			return nil, fmt.Errorf("unexpected %s", url)
		}
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	got, err := os.ReadFile(dest)
	if err != nil {
		t.Fatal(err)
	}
	if string(got) != string(payload) {
		t.Fatalf("got %q", got)
	}
	if !strings.Contains(errb.String(), "updated") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunUpdateRejectsBadChecksum(t *testing.T) {
	dir := t.TempDir()
	dest := filepath.Join(dir, "wheretoken")
	if err := os.WriteFile(dest, []byte("old"), 0o755); err != nil {
		t.Fatal(err)
	}
	app, _, errb := testApp([]string{"update"})
	app.GOOS = "darwin"
	app.GOARCH = "arm64"
	app.Executable = func() (string, error) { return dest, nil }
	app.HTTPGet = func(url string) ([]byte, error) {
		if strings.HasSuffix(url, "checksums.txt") {
			return []byte("deadbeef  wheretoken_darwin_arm64.tar.gz\n"), nil
		}
		return tarGzNamed(t, "wheretoken", []byte("x")), nil
	}
	if code := app.Run(); code != ExitFail {
		t.Fatalf("code=%d", code)
	}
	got, _ := os.ReadFile(dest)
	if string(got) != "old" {
		t.Fatal("must not replace on checksum fail")
	}
	if !strings.Contains(errb.String(), "checksum") {
		t.Fatalf("stderr=%s", errb.String())
	}
}

func TestRunUpdateUsesBrewWhenCellar(t *testing.T) {
	app, _, errb := testApp([]string{"update"})
	var got []string
	app.Executable = func() (string, error) {
		return "/opt/homebrew/Cellar/wheretoken/0.3.0/bin/wheretoken", nil
	}
	app.RunCmd = func(name string, args ...string) error {
		got = append(got, name+" "+strings.Join(args, " "))
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if len(got) != 1 || got[0] != "brew upgrade wheretoken" {
		t.Fatalf("got=%v", got)
	}
}

func TestRunUninstallUsesBrewWhenCellar(t *testing.T) {
	app, _, errb := testApp([]string{"uninstall"})
	var got []string
	app.Executable = func() (string, error) {
		return "/opt/homebrew/Cellar/wheretoken/0.3.0/bin/wheretoken", nil
	}
	app.RunCmd = func(name string, args ...string) error {
		got = append(got, name+" "+strings.Join(args, " "))
		return nil
	}
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if len(got) != 1 || got[0] != "brew uninstall wheretoken" {
		t.Fatalf("got=%v", got)
	}
}

func TestBrewManaged(t *testing.T) {
	if !brewManaged("/opt/homebrew/Cellar/wheretoken/0.3.0/bin/wheretoken") {
		t.Fatal("cellar")
	}
	if brewManaged("/Users/x/.local/bin/wheretoken") {
		t.Fatal("local")
	}
}

func tarGzNamed(t *testing.T, name string, payload []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	gz := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gz)
	if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0755, Size: int64(len(payload))}); err != nil {
		t.Fatal(err)
	}
	if _, err := tw.Write(payload); err != nil {
		t.Fatal(err)
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := gz.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}
