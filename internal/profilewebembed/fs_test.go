package profilewebembed

import (
	"strings"
	"testing"
)

func TestLivePageHasNoDashboardAPIs(t *testing.T) {
	js, err := Read("assets/profile.js")
	if err != nil {
		t.Fatal(err)
	}
	s := string(js)
	for _, bad := range []string{"/api/summary", "/api/v1/", "wheretoken login", "wheretoken sync", "community"} {
		if strings.Contains(s, bad) {
			t.Fatalf("forbidden %q", bad)
		}
	}
	if !strings.Contains(s, "./profile.json") {
		t.Fatal("must fetch ./profile.json")
	}
}

func TestNewsprintPaperIsEmbeddedAndProcedural(t *testing.T) {
	asset, err := Read("assets/newsprint-paper.svg")
	if err != nil {
		t.Fatal(err)
	}
	s := string(asset)
	for _, want := range []string{"feTurbulence", "fractalNoise"} {
		if !strings.Contains(s, want) {
			t.Fatalf("newsprint paper is missing %q", want)
		}
	}
	folds, err := Read("assets/newsprint-folds.svg")
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{`id="vertical-fold"`, `id="diagonal-fold"`} {
		if !strings.Contains(string(folds), want) {
			t.Fatalf("newsprint folds are missing %q", want)
		}
	}
	for _, bad := range []string{`href="http://`, `href="https://`, `href='http://`, `href='https://`} {
		if strings.Contains(s, bad) {
			t.Fatalf("newsprint paper must not load external asset %q", bad)
		}
	}
}
