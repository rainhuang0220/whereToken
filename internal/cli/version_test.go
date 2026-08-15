package cli

import "testing"

func TestResolveVersionPrefersLdflag(t *testing.T) {
	if got := ResolveVersion("1.2.3"); got != "1.2.3" {
		t.Fatal(got)
	}
	if got := ResolveVersion(""); got == "" {
		t.Fatal("empty")
	}
}

func TestTidyPseudoVersion(t *testing.T) {
	if got := tidyVersion("v0.0.0-20260815190153-a625cbfa65ee+dirty"); got != "dev-a625cbf" {
		t.Fatalf("got %q", got)
	}
	if got := tidyVersion("1.2.3"); got != "1.2.3" {
		t.Fatalf("got %q", got)
	}
	if got := tidyVersion("v1.0.0"); got != "v1.0.0" {
		t.Fatalf("got %q", got)
	}
}
