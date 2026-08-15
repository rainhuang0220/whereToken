package cli

import (
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestCheckedInCompletionsMatchGenerator(t *testing.T) {
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	root := filepath.Join(filepath.Dir(file), "..", "..", "completions")
	cases := []struct{ shell, name string }{
		{"bash", "wheretoken.bash"},
		{"zsh", "_wheretoken"},
		{"fish", "wheretoken.fish"},
		{"powershell", "wheretoken.ps1"},
	}
	for _, c := range cases {
		want, err := Completion(c.shell)
		if err != nil {
			t.Fatal(err)
		}
		got, err := os.ReadFile(filepath.Join(root, c.name))
		if err != nil {
			t.Fatalf("read %s: %v", c.name, err)
		}
		if string(got) != want {
			t.Fatalf("%s drifted from cli.Completion", c.name)
		}
	}
}
