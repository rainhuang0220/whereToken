package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/rainhuang0220/whereToken/internal/adapter/testhome"
)

func TestRunHomeFixturePrintsKimiTable(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dst := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	app, out, errb := testApp([]string{"--home", dir, "--ascii"})
	app.Scan = nil
	app.Home = testhome.New(dir)
	app.StderrTTY = false
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	s := out.String()
	if strings.Contains(s, "eyJ") {
		t.Fatal("jwt")
	}
	if !strings.Contains(s, "0.0012 M") && !strings.Contains(s, "Kimi") {
		t.Fatalf("expected kimi fixture table:\n%s", s)
	}
	if strings.Contains(s, "/Users/rainhuang") {
		t.Fatal("leaked owner home")
	}
}

func TestRunKimiAliasOnFixture(t *testing.T) {
	t.Setenv("WHERETOKEN_EXTRA_ROOTS", "")
	dir := t.TempDir()
	dst := filepath.Join(dir, ".kimi-code", "sessions", "x", "s", "agents", "main")
	if err := os.MkdirAll(dst, 0o755); err != nil {
		t.Fatal(err)
	}
	_, file, _, _ := runtime.Caller(0)
	src := filepath.Join(filepath.Dir(file), "..", "..", "testdata", "adapters", "kimi", "session", "agents", "main", "wire.jsonl")
	data, err := os.ReadFile(src)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dst, "wire.jsonl"), data, 0o644); err != nil {
		t.Fatal(err)
	}
	app, out, errb := testApp([]string{"--kimi", "--home", dir, "--ascii"})
	app.Scan = nil
	if code := app.Run(); code != ExitOK {
		t.Fatalf("code=%d %s", code, errb.String())
	}
	if !strings.Contains(out.String(), "Kimi") || !strings.Contains(out.String(), "0.0012 M") {
		t.Fatalf("%s", out.String())
	}
}
