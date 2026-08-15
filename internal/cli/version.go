package cli

import (
	"runtime/debug"
	"testing"
)

func ResolveVersion(ldflag string) string {
	if ldflag != "" && ldflag != "dev" {
		return ldflag
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		if v := info.Main.Version; v != "" && v != "(devel)" {
			return v
		}
	}
	if ldflag == "" {
		return "dev"
	}
	return ldflag
}

func TestResolveVersionPrefersLdflag(t *testing.T) {
	if got := ResolveVersion("1.2.3"); got != "1.2.3" {
		t.Fatal(got)
	}
	if got := ResolveVersion(""); got == "" {
		t.Fatal("empty")
	}
}
